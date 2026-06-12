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
	analyzer.NewForge = func(context.Context) analyze.Forge {
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

type failingForge struct{}

func (failingForge) PullRequestForCommit(context.Context, github.Repository, string) (*model.Reference, error) {
	return nil, errors.New("network down")
}

func (failingForge) Issue(context.Context, github.Repository, int) (model.Reference, error) {
	return model.Reference{}, errors.New("network down")
}

func temporaryRepository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nconst enabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "Prevent duplicate work")
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
