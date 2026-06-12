package github

import (
	"fmt"
	"net/url"
	"strings"
)

type Repository struct {
	Owner string
	Name  string
}

func ParseRemote(raw string) (Repository, error) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "git@github.com:") {
		return repositoryFromPath(strings.TrimPrefix(value, "git@github.com:"))
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return Repository{}, fmt.Errorf("invalid remote URL %q", raw)
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return Repository{}, fmt.Errorf("remote host %q is not GitHub", parsed.Hostname())
	}
	return repositoryFromPath(strings.TrimPrefix(parsed.Path, "/"))
}

func repositoryFromPath(path string) (Repository, error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repository{}, fmt.Errorf("cannot identify GitHub owner/repository from %q", path)
	}
	return Repository{Owner: parts[0], Name: parts[1]}, nil
}
