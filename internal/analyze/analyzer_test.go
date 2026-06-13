package analyze_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0stick/CodeWhy/internal/analyze"
	"github.com/0stick/CodeWhy/internal/github"
	"github.com/0stick/CodeWhy/internal/model"
	"github.com/0stick/CodeWhy/internal/target"
)

func TestExtractIssueReferences(t *testing.T) {
	got := analyze.ExtractIssueReferences("Fixes #12, closes: #7. See #12 and resolves #99.")
	want := []int{7, 12, 99}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDetermineReasonConfidence(t *testing.T) {
	tests := []struct {
		name       string
		result     model.Result
		confidence string
	}{
		{"pull request", model.Result{PullRequest: &model.Reference{Title: "Prevent duplicate refreshes"}}, "high"},
		{"issue", model.Result{Issues: []model.Reference{{Title: "Tokens are invalidated"}}}, "high"},
		{"commit", model.Result{Commit: model.Commit{Message: "Prevent duplicate token refresh requests"}}, "medium"},
		{"undocumented", model.Result{Commit: model.Commit{Message: "wip"}}, "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, confidence := analyze.DetermineReason(tt.result)
			if confidence != tt.confidence {
				t.Fatalf("got %q, want %q", confidence, tt.confidence)
			}
		})
	}
}

func TestNetworkFailureDegradesToLocalResult(t *testing.T) {
	repo := temporaryRepository(t)
	runGit(t, repo, "remote", "add", "origin", "https://github.com/acme/tool.git")

	analyzer := analyze.New(repo)
	analyzer.NewForge = func(context.Context, github.Repository) analyze.Forge {
		return failingForge{}
	}
	result, err := analyzer.Explain(context.Background(), target.Location{File: "main.go", Line: 3}, analyze.Options{Remote: "origin"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Commit.Message != "Prevent duplicate work" {
		t.Fatalf("local commit missing: %#v", result.Commit)
	}
	if result.Confidence != "medium" || len(result.Warnings) != 1 {
		t.Fatalf("unexpected degraded result: %#v", result)
	}
}

func TestPullRequestReferenceIsNotAddedAsIssue(t *testing.T) {
	repo := temporaryRepositoryWithMessage(t, "wip\n\nSee #12")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/acme/tool.git")

	analyzer := analyze.New(repo)
	analyzer.NewForge = func(context.Context, github.Repository) analyze.Forge {
		return pullRequestIssueForge{}
	}
	result, err := analyzer.Explain(context.Background(), target.Location{File: "main.go", Line: 3}, analyze.Options{Remote: "origin"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("pull request was added as issue: %#v", result.Issues)
	}
	if result.Confidence != "low" {
		t.Fatalf("confidence = %q, want low", result.Confidence)
	}
}

func TestAnalyzerSelectsBestPullRequestAndKeepsCandidates(t *testing.T) {
	repo := temporaryRepositoryWithMessage(t, "wip")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/acme/tool.git")

	analyzer := analyze.New(repo)
	analyzer.NewForge = func(context.Context, github.Repository) analyze.Forge {
		return multiplePullRequestForge{}
	}
	result, err := analyzer.Explain(context.Background(), target.Location{File: "main.go", Line: 3}, analyze.Options{Remote: "origin"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.PullRequests) != 2 {
		t.Fatalf("candidates = %#v", result.PullRequests)
	}
	if result.PullRequest == nil || result.PullRequest.Number != 20 {
		t.Fatalf("selected pull request = %#v", result.PullRequest)
	}
}

func TestAnalyzerReportsContainingGoFunction(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")
	code := "package main\n\nfunc run() {\n\tprintln(\"running\")\n}\n"
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "Add run function")

	analyzer := analyze.New(repo)
	result, err := analyzer.Explain(context.Background(), target.Location{File: "main.go", Line: 4}, analyze.Options{
		Offline:     true,
		Function:    true,
		IncludeDiff: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target == nil || result.Target.Function == nil || result.Target.Function.Name != "run" {
		t.Fatalf("unexpected target: %#v", result.Target)
	}
	if result.Target.SourceLine != 3 {
		t.Fatalf("source line = %d, want function start line 3", result.Target.SourceLine)
	}
}

type failingForge struct{}

func (failingForge) PullRequestsForCommit(context.Context, github.Repository, string) ([]model.Reference, error) {
	return nil, errors.New("network down")
}

func (failingForge) Issue(context.Context, github.Repository, int) (model.Reference, error) {
	return model.Reference{}, errors.New("network down")
}

type pullRequestIssueForge struct{}

func (pullRequestIssueForge) PullRequestsForCommit(context.Context, github.Repository, string) ([]model.Reference, error) {
	return []model.Reference{}, nil
}

func (pullRequestIssueForge) Issue(context.Context, github.Repository, int) (model.Reference, error) {
	return model.Reference{Number: 12, Title: "A pull request", Kind: "pull_request"}, nil
}

type multiplePullRequestForge struct{}

func (multiplePullRequestForge) PullRequestsForCommit(context.Context, github.Repository, string) ([]model.Reference, error) {
	return []model.Reference{
		{Number: 10, Title: "Backport", Kind: "pull_request", Merged: true, BaseBranch: "release"},
		{Number: 20, Title: "Primary fix", Kind: "pull_request", Merged: true, BaseBranch: "main", BaseDefault: true},
	}, nil
}

func (multiplePullRequestForge) Issue(context.Context, github.Repository, int) (model.Reference, error) {
	return model.Reference{}, errors.New("unexpected issue request")
}

func temporaryRepository(t *testing.T) string {
	return temporaryRepositoryWithMessage(t, "Prevent duplicate work")
}

func temporaryRepositoryWithMessage(t *testing.T, message string) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nconst enabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", message)
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
