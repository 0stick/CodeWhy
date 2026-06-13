package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0stick/CodeWhy/internal/github"
)

func TestResolveTokenPrefersCodewhyEnvironmentVariable(t *testing.T) {
	t.Setenv("CODEWHY_GITHUB_TOKEN", "codewhy-token")
	t.Setenv("GITHUB_TOKEN", "github-token")
	if got := github.ResolveToken(context.Background()); got != "codewhy-token" {
		t.Fatalf("got %q", got)
	}
}

func TestClientGetsPullRequestsAndSelectsMergedDefaultBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/tool/commits/abc/pulls":
			_, _ = w.Write([]byte(`[
				{"number":7,"title":"Backport","html_url":"https://github.com/acme/tool/pull/7","state":"closed","merged_at":"2024-01-01T00:00:00Z","base":{"ref":"release","repo":{"default_branch":"main"}}},
				{"number":9,"title":"Primary fix","body":"Fixes #5","html_url":"https://github.com/acme/tool/pull/9","state":"closed","merged_at":"2024-01-02T00:00:00Z","base":{"ref":"main","repo":{"default_branch":"main"}}},
				{"number":11,"title":"Open follow-up","html_url":"https://github.com/acme/tool/pull/11","state":"open","merged_at":null,"base":{"ref":"main","repo":{"default_branch":"main"}}}
			]`))
		case "/repos/acme/tool/issues/5":
			_, _ = w.Write([]byte(`{"number":5,"title":"Concurrent refresh fails","body":"Tokens become invalid.","html_url":"https://github.com/acme/tool/issues/5","state":"closed"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := github.NewClient("secret")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	repo := github.Repository{Owner: "acme", Name: "tool"}

	candidates, err := client.PullRequestsForCommit(context.Background(), repo, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 3 {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}
	pr := github.SelectPullRequest(candidates)
	if pr == nil || pr.Number != 9 || pr.Title != "Primary fix" {
		t.Fatalf("unexpected selected PR: %#v", pr)
	}

	issue, err := client.Issue(context.Background(), repo, 5)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != 5 || issue.Title != "Concurrent refresh fails" {
		t.Fatalf("unexpected issue: %#v", issue)
	}
}

func TestIssueMarksPullRequestReferences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":12,"title":"This is a PR","html_url":"https://github.com/acme/tool/pull/12","state":"closed","pull_request":{}}`))
	}))
	defer server.Close()

	client := github.NewClient("")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	ref, err := client.Issue(context.Background(), github.Repository{Owner: "acme", Name: "tool"}, 12)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Kind != "pull_request" {
		t.Fatalf("kind = %q, want pull_request", ref.Kind)
	}
}

func TestClientReportsNetworkFailure(t *testing.T) {
	client := github.NewClient("")
	client.BaseURL = "http://127.0.0.1:1"
	if _, err := client.PullRequestsForCommit(context.Background(), github.Repository{Owner: "a", Name: "b"}, "abc"); err == nil {
		t.Fatal("expected network error")
	}
}
