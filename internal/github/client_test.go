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

func TestClientGetsPullRequestAndIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/tool/commits/abc/pulls":
			_, _ = w.Write([]byte(`[{"number":7,"title":"Prevent races","body":"Fixes #5","html_url":"https://github.com/acme/tool/pull/7","state":"closed"}]`))
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

	pr, err := client.PullRequestForCommit(context.Background(), repo, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if pr == nil || pr.Number != 7 || pr.Title != "Prevent races" {
		t.Fatalf("unexpected PR: %#v", pr)
	}

	issue, err := client.Issue(context.Background(), repo, 5)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != 5 || issue.Title != "Concurrent refresh fails" {
		t.Fatalf("unexpected issue: %#v", issue)
	}
}

func TestClientReportsNetworkFailure(t *testing.T) {
	client := github.NewClient("")
	client.BaseURL = "http://127.0.0.1:1"
	if _, err := client.PullRequestForCommit(context.Background(), github.Repository{Owner: "a", Name: "b"}, "abc"); err == nil {
		t.Fatal("expected network error")
	}
}
