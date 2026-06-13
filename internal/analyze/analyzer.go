package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	gitclient "github.com/0stick/CodeWhy/internal/git"
	"github.com/0stick/CodeWhy/internal/github"
	"github.com/0stick/CodeWhy/internal/model"
	"github.com/0stick/CodeWhy/internal/source"
	"github.com/0stick/CodeWhy/internal/target"
)

var issuePattern = regexp.MustCompile(`(?i)(?:\b(?:fix(?:e[sd])?|close[sd]?|resolve[sd]?)\s*:?\s*)?#([1-9][0-9]*)\b`)

type Forge interface {
	PullRequestsForCommit(context.Context, github.Repository, string) ([]model.Reference, error)
	Issue(context.Context, github.Repository, int) (model.Reference, error)
}

type Options struct {
	Remote       string
	Offline      bool
	Context      int
	IncludeDiff  bool
	MaxDiffBytes int
	History      bool
	Function     bool
	GitHubHost   string
	NoCache      bool
	CacheTTL     time.Duration
	Verbose      func(string)
}

type Analyzer struct {
	Git      gitclient.Client
	NewForge func(context.Context, github.Repository) Forge
}

func New(dir string) *Analyzer {
	return &Analyzer{
		Git: gitclient.Client{Dir: dir},
		NewForge: func(ctx context.Context, repo github.Repository) Forge {
			return github.NewClientForRepository(github.ResolveTokenForHost(ctx, repo.Host), repo)
		},
	}
}

func (a *Analyzer) Explain(ctx context.Context, location target.Location, options Options) (model.Result, error) {
	logf(options, "checking Git repository")
	root, err := a.Git.Root(ctx)
	if err != nil {
		return model.Result{}, err
	}
	file, err := repositoryPath(root, location.File)
	if err != nil {
		return model.Result{}, err
	}

	analysisLine := location.Line
	var function *model.Function
	if options.Function {
		function, err = source.FindFunction(root, file, location.Line)
		if err != nil {
			return model.Result{}, err
		}
		analysisLine = function.StartLine
		logf(options, "target belongs to function %s at lines %d-%d", function.Name, function.StartLine, function.EndLine)
	}

	logf(options, "tracing %s:%d with git blame -w -M -C", file, analysisLine)
	blame, err := a.Git.BlameLine(ctx, file, analysisLine)
	if err != nil {
		return model.Result{}, err
	}
	commit, err := a.Git.CommitWithOptions(ctx, blame.SHA, gitclient.CommitOptions{
		IncludeDiff:  options.IncludeDiff,
		MaxDiffBytes: options.MaxDiffBytes,
	})
	if err != nil {
		return model.Result{}, err
	}
	var contextLines []model.ContextLine
	if options.Context > 0 {
		contextLines, err = gitclient.ReadContext(root, file, location.Line, options.Context)
		if err != nil {
			return model.Result{}, err
		}
	}

	result := baseResult(commit)
	result.Target = &model.Target{
		File:       filepath.ToSlash(file),
		Line:       location.Line,
		Function:   function,
		SourceFile: filepath.ToSlash(blame.SourceFile),
		SourceLine: blame.SourceLine,
		Code:       blame.Code,
		Context:    contextLines,
	}
	if options.History {
		logf(options, "tracing complete line history")
		result.History, err = a.Git.LineHistory(ctx, file, analysisLine)
		if err != nil {
			return model.Result{}, err
		}
	}
	a.enrich(ctx, &result, options)
	return result, nil
}

func (a *Analyzer) ExplainCommit(ctx context.Context, sha string, options Options) (model.Result, error) {
	if _, err := a.Git.Root(ctx); err != nil {
		return model.Result{}, err
	}
	logf(options, "reading commit %s", sha)
	commit, err := a.Git.CommitWithOptions(ctx, sha, gitclient.CommitOptions{
		IncludeDiff:  options.IncludeDiff,
		MaxDiffBytes: options.MaxDiffBytes,
	})
	if err != nil {
		return model.Result{}, err
	}
	result := baseResult(commit)
	a.enrich(ctx, &result, options)
	return result, nil
}

func (a *Analyzer) enrich(ctx context.Context, result *model.Result, options Options) {
	if options.Offline {
		setReason(result)
		return
	}

	logf(options, "inspecting remote %s", options.Remote)
	remoteURL, err := a.Git.RemoteURL(ctx, options.Remote)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		setReason(result)
		return
	}
	repo, err := github.ParseRemoteForHost(remoteURL, options.GitHubHost)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		setReason(result)
		return
	}
	result.Commit.URL = fmt.Sprintf("%s/%s/%s/commit/%s", repo.WebBaseURL(), repo.Owner, repo.Name, result.Commit.SHA)
	for index := range result.History {
		result.History[index].URL = fmt.Sprintf("%s/%s/%s/commit/%s", repo.WebBaseURL(), repo.Owner, repo.Name, result.History[index].SHA)
	}

	forge := a.NewForge(ctx, repo)
	if client, ok := forge.(*github.Client); ok {
		client.DisableCache = options.NoCache
		client.CacheTTL = options.CacheTTL
	}
	logf(options, "querying GitHub for associated pull request")
	pullRequests, err := forge.PullRequestsForCommit(ctx, repo, result.Commit.SHA)
	if err != nil {
		result.Warnings = append(result.Warnings, "GitHub enrichment unavailable: "+err.Error())
		setReason(result)
		return
	}
	result.PullRequests = pullRequests
	pr := github.SelectPullRequest(pullRequests)
	result.PullRequest = pr

	text := result.Commit.Message
	if pr != nil {
		text += "\n" + pr.Title + "\n" + pr.Body
	}
	numbers := ExtractIssueReferences(text)
	for _, number := range numbers {
		if pr != nil && number == pr.Number {
			continue
		}
		issue, issueErr := forge.Issue(ctx, repo, number)
		if issueErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not load issue #%d: %v", number, issueErr))
			continue
		}
		if issue.Kind == "pull_request" {
			continue
		}
		result.Issues = append(result.Issues, issue)
	}
	setReason(result)
}

func ExtractIssueReferences(text string) []int {
	matches := issuePattern.FindAllStringSubmatch(text, -1)
	seen := make(map[int]bool)
	var result []int
	for _, match := range matches {
		var number int
		_, _ = fmt.Sscanf(match[1], "%d", &number)
		if number > 0 && !seen[number] {
			seen[number] = true
			result = append(result, number)
		}
	}
	sort.Ints(result)
	return result
}

func DetermineReason(result model.Result) (string, string) {
	if result.PullRequest != nil && strings.TrimSpace(result.PullRequest.Title+result.PullRequest.Body) != "" {
		return summarize(result.PullRequest.Title, result.PullRequest.Body), "high"
	}
	for _, issue := range result.Issues {
		if strings.TrimSpace(issue.Title+issue.Body) != "" {
			return summarize(issue.Title, issue.Body), "high"
		}
	}
	if clearCommitMessage(result.Commit.Message) {
		return summarize(firstLine(result.Commit.Message), remainingLines(result.Commit.Message)), "medium"
	}
	return "No documented reason found", "low"
}

func setReason(result *model.Result) {
	result.Reason, result.Confidence = DetermineReason(*result)
}

func baseResult(commit model.Commit) model.Result {
	return model.Result{
		SchemaVersion: model.SchemaVersion,
		Commit:        commit,
		PullRequests:  []model.Reference{},
		Issues:        []model.Reference{},
		History:       []model.HistoryEntry{},
		Warnings:      []string{},
	}
}

func summarize(title, body string) string {
	title = strings.TrimSpace(title)
	body = firstParagraph(body)
	if body == "" {
		return title
	}
	if title == "" {
		return body
	}
	if strings.EqualFold(strings.TrimRight(title, ".!?"), strings.TrimRight(body, ".!?")) {
		return title
	}
	return strings.TrimRight(title, ".") + ". " + body
}

func firstParagraph(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	for _, paragraph := range strings.Split(value, "\n\n") {
		if trimmed := strings.TrimSpace(paragraph); trimmed != "" {
			return strings.Join(strings.Fields(trimmed), " ")
		}
	}
	return ""
}

func firstLine(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	return strings.TrimSpace(lines[0])
}

func remainingLines(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return ""
	}
	return strings.Join(lines[1:], "\n")
}

func clearCommitMessage(message string) bool {
	subject := strings.TrimSpace(firstLine(message))
	if subject == "" {
		return false
	}
	lower := strings.ToLower(subject)
	generic := []string{"wip", "update", "changes", "fix", "misc", "initial commit"}
	for _, item := range generic {
		if lower == item {
			return false
		}
	}
	return len([]rune(subject)) >= 8 && strings.IndexFunc(subject, unicode.IsLetter) >= 0
}

func repositoryPath(root, input string) (string, error) {
	path := filepath.Clean(input)
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(path), nil
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("target file %q is outside repository %s", input, root)
	}
	return filepath.ToSlash(relative), nil
}

func logf(options Options, format string, args ...any) {
	if options.Verbose != nil {
		options.Verbose(fmt.Sprintf(format, args...))
	}
}
