package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/0stick/CodeWhy/internal/model"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

func NewClient(token string) *Client {
	return &Client{
		BaseURL: "https://api.github.com",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Token: token,
	}
}

func ResolveToken(ctx context.Context) string {
	if token := strings.TrimSpace(os.Getenv("CODEWHY_GITHUB_TOKEN")); token != "" {
		return token
	}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	tokenContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tokenContext, "gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (c *Client) PullRequestsForCommit(ctx context.Context, repo Repository, sha string) ([]model.Reference, error) {
	path := "/repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name) +
		"/commits/" + url.PathEscape(sha) + "/pulls"
	var payload []apiReference
	if err := c.get(ctx, path, &payload, "application/vnd.github+json"); err != nil {
		return nil, err
	}
	result := make([]model.Reference, 0, len(payload))
	for _, candidate := range payload {
		result = append(result, candidate.model("pull_request"))
	}
	return result, nil
}

func (c *Client) Issue(ctx context.Context, repo Repository, number int) (model.Reference, error) {
	path := "/repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name) +
		"/issues/" + strconv.Itoa(number)
	var payload apiReference
	if err := c.get(ctx, path, &payload, "application/vnd.github+json"); err != nil {
		return model.Reference{}, err
	}
	return payload.model(payload.kind()), nil
}

func (c *Client) get(ctx context.Context, path string, target any, accept string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return fmt.Errorf("create GitHub request: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "codewhy")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	response, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub request failed: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("GitHub API returned %s: %s", response.Status, message)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

type apiReference struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	State       string    `json:"state"`
	MergedAt    *string   `json:"merged_at"`
	PullRequest *struct{} `json:"pull_request"`
	Base        struct {
		Ref  string `json:"ref"`
		Repo struct {
			DefaultBranch string `json:"default_branch"`
		} `json:"repo"`
	} `json:"base"`
}

func (r apiReference) model(kind string) model.Reference {
	return model.Reference{
		Number:      r.Number,
		Title:       r.Title,
		Body:        r.Body,
		URL:         r.HTMLURL,
		State:       r.State,
		Kind:        kind,
		Merged:      r.MergedAt != nil,
		BaseBranch:  r.Base.Ref,
		BaseDefault: r.Base.Ref != "" && r.Base.Ref == r.Base.Repo.DefaultBranch,
	}
}

func (r apiReference) kind() string {
	if r.PullRequest != nil {
		return "pull_request"
	}
	return "issue"
}

func SelectPullRequest(candidates []model.Reference) *model.Reference {
	if len(candidates) == 0 {
		return nil
	}
	best := 0
	bestScore := -1
	for index, candidate := range candidates {
		score := 0
		if candidate.BaseDefault {
			score += 1
		}
		if candidate.Merged {
			score += 2
		}
		if score > bestScore {
			best = index
			bestScore = score
		}
	}
	selected := candidates[best]
	return &selected
}
