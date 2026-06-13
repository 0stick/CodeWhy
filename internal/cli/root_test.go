package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/0stick/CodeWhy/internal/cli"
	"github.com/0stick/CodeWhy/internal/model"
)

func TestJSONErrorForInvalidTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.ExecuteArgs([]string{"--json", "invalid"}, &stdout, &stderr)
	if err == nil || !cli.ErrorWasRendered(err) {
		t.Fatalf("expected rendered command error, got %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var response model.ErrorResponse
	if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.SchemaVersion != model.SchemaVersion || response.Error.Code != "invalid_target" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestJSONErrorOutsideGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.ExecuteArgs([]string{"--json", "--offline", "main.go:1"}, &stdout, &stderr)
	if err == nil || !cli.ErrorWasRendered(err) {
		t.Fatalf("expected rendered command error, got %v", err)
	}

	var response model.ErrorResponse
	if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.Error.Code != "not_git_repository" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestRepoOptionAnalyzesRepositoryOutsideWorkingDirectory(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nconst enabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "Add feature toggle")

	t.Chdir(t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.ExecuteArgs([]string{"--json", "--offline", "--repo", repo, "--include-diff=false", "main.go:3"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	var result model.Result
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if result.Target == nil || result.Target.File != "main.go" || result.Commit.DiffIncluded {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
