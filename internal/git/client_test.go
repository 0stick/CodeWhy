package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitclient "github.com/0stick/CodeWhy/internal/git"
)

func TestClientReadsBlameAndCommitFromTemporaryRepository(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")

	path := filepath.Join(repo, "hello world.go")
	if err := os.WriteFile(path, []byte("package hello\n\nconst Answer = 42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "hello world.go")
	runGit(t, repo, "commit", "-m", "Explain the answer")

	client := gitclient.Client{Dir: repo}
	blame, err := client.BlameLine(context.Background(), "hello world.go", 3)
	if err != nil {
		t.Fatal(err)
	}
	if blame.Code != "const Answer = 42" {
		t.Fatalf("unexpected code: %q", blame.Code)
	}

	commit, err := client.Commit(context.Background(), blame.SHA)
	if err != nil {
		t.Fatal(err)
	}
	if commit.Author != "Test Author" || commit.Message != "Explain the answer" {
		t.Fatalf("unexpected commit: %#v", commit)
	}
	if len(commit.Files) != 1 || commit.Files[0] != "hello world.go" {
		t.Fatalf("unexpected files: %#v", commit.Files)
	}
}

func TestBlameLinePreservesUnicodeSourceFileAfterRename(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")

	source := filepath.Join(repo, "源文件.go")
	if err := os.WriteFile(source, []byte("package hello\n\nconst Answer = 42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "源文件.go")
	runGit(t, repo, "commit", "-m", "Add answer")
	runGit(t, repo, "mv", "源文件.go", "重命名.go")
	runGit(t, repo, "commit", "-m", "Rename source file")

	client := gitclient.Client{Dir: repo}
	blame, err := client.BlameLine(context.Background(), "重命名.go", 3)
	if err != nil {
		t.Fatal(err)
	}
	if blame.SourceFile != "源文件.go" {
		t.Fatalf("source file = %q, want %q", blame.SourceFile, "源文件.go")
	}
}

func TestCommitLimitsLargeDiff(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test Author")
	runGit(t, repo, "config", "user.email", "test@example.com")

	content := strings.Repeat("large diff line with enough content\n", gitclient.MaxDiffBytes/16)
	if err := os.WriteFile(filepath.Join(repo, "large.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "large.txt")
	runGit(t, repo, "commit", "-m", "Add large generated file")

	client := gitclient.Client{Dir: repo}
	commit, err := client.Commit(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !commit.DiffTruncated {
		t.Fatal("expected large diff to be marked as truncated")
	}
	if len(commit.Diff) > gitclient.MaxDiffBytes {
		t.Fatalf("diff has %d bytes, limit is %d", len(commit.Diff), gitclient.MaxDiffBytes)
	}
}

func TestReadContext(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "sample.txt"), []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := gitclient.ReadContext(repo, "sample.txt", 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 || !lines[1].Current || lines[1].Code != "two" {
		t.Fatalf("unexpected context: %#v", lines)
	}
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
