package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	gitclient "github.com/0stick/CodeWhy/internal/git"
	"github.com/0stick/CodeWhy/internal/github"
	"github.com/0stick/CodeWhy/internal/model"
	"github.com/0stick/CodeWhy/internal/target"
)

var issuePattern = regexp.MustCompile(`(?i)(?:\b(?:fix(?:e[sd])?|close[sd]?|resolve[sd]?)\s*:?\s*)?#([1-9][0-9]*)\b`)

type Forge interface {
	PullRequestForCommit(context.Context, github.Repository, string) (*model.Reference, error)
	Issue(context.Context, github.Repository, int) (model.Reference, error)
}

type Options struct {
	Remote  string
	Offline bool
	Context int
	Verbose func(string)
}

type Analyzer struct {
	Git      gitclient.Client
	NewForge func(context.Context) Forge
}

func New(dir string) *Analyzer {
	return &Analyzer{
		Git: gitclient.Client{Dir: dir},
		NewForge: func(ctx context.Context) Forge {
			return github.NewClient(github.ResolveToken(ctx))
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

	logf(options, "tracing %s:%d with git blame -w -M -C", file, location.Line)
	blame, err := a.Git.BlameLine(ctx, file, location.Line)
	if err != nil {
		return model.Result{}, err
	}
	commit, err := a.Git.Commit(ctx, blame.SHA)
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
		SourceFile: filepath.ToSlash(blame.SourceFile),
		SourceLine: blame.SourceLine,
		Code:       blame.Code,
		Context:    contextLines,
	}
	a.enrich(ctx, &result, options)
	return result, nil
}

func (a *Analyzer) ExplainCommit(ctx context.Context, sha string, options Options) (model.Result, error) {
	if _, err := a.Git.Root(ctx); err != nil {
		return model.Result{}, err
	}
	logf(options, "reading commit %s", sha)
	commit, err := a.Git.Commit(ctx, sha)
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
	repo, err := github.ParseRemote(remoteURL)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		setReason(result)
		return
	}
	result.Commit.URL = fmt.Sprintf("https://github.com/%s/%s/commit/%s", repo.Owner, repo.Name, result.Commit.SHA)

	forge := a.NewForge(ctx)
	logf(options, "querying GitHub for associated pull request")
	pr, err := forge.PullRequestForCommit(ctx, repo, result.Commit.SHA)
	if err != nil {
		result.Warnings = append(result.Warnings, "GitHub enrichment unavailable: "+err.Error())
		setReason(result)
		return
	}
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
		Issues:        []model.Reference{},
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
