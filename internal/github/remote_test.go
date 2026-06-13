package github_test

import (
	"testing"

	"github.com/0stick/CodeWhy/internal/github"
)

func TestParseRemote(t *testing.T) {
	tests := map[string]github.Repository{
		"git@github.com:owner/repo.git":       {Host: "github.com", Owner: "owner", Name: "repo"},
		"https://github.com/owner/repo.git":   {Host: "github.com", Owner: "owner", Name: "repo"},
		"ssh://git@github.com/owner/repo.git": {Host: "github.com", Owner: "owner", Name: "repo"},
	}
	for raw, want := range tests {
		got, err := github.ParseRemote(raw)
		if err != nil {
			t.Fatalf("ParseRemote(%q): %v", raw, err)
		}
		if got != want {
			t.Errorf("ParseRemote(%q) = %#v, want %#v", raw, got, want)
		}
	}
}

func TestParseGitHubEnterpriseRemote(t *testing.T) {
	got, err := github.ParseRemoteForHost("git@git.example.com:owner/repo.git", "git.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "git.example.com" || got.APIBaseURL() != "https://git.example.com/api/v3" {
		t.Fatalf("unexpected repository: %#v", got)
	}
}

func TestParseRemoteRejectsNonGitHub(t *testing.T) {
	if _, err := github.ParseRemote("https://gitlab.com/owner/repo.git"); err == nil {
		t.Fatal("expected error")
	}
}
