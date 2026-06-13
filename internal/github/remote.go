package github

import (
	"fmt"
	"net/url"
	"strings"
)

type Repository struct {
	Host  string
	Owner string
	Name  string
}

func ParseRemote(raw string) (Repository, error) {
	return ParseRemoteForHost(raw, "github.com")
}

func ParseRemoteForHost(raw, host string) (Repository, error) {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		host = "github.com"
	}
	value := strings.TrimSpace(raw)
	scpPrefix := "git@" + host + ":"
	if strings.HasPrefix(strings.ToLower(value), scpPrefix) {
		return repositoryFromPath(host, value[len(scpPrefix):])
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return Repository{}, fmt.Errorf("invalid remote URL %q", raw)
	}
	if !strings.EqualFold(parsed.Hostname(), host) {
		return Repository{}, fmt.Errorf("remote host %q does not match configured GitHub host %q", parsed.Hostname(), host)
	}
	return repositoryFromPath(host, strings.TrimPrefix(parsed.Path, "/"))
}

func (r Repository) APIBaseURL() string {
	if strings.EqualFold(r.Host, "github.com") || r.Host == "" {
		return "https://api.github.com"
	}
	return "https://" + r.Host + "/api/v3"
}

func (r Repository) WebBaseURL() string {
	host := r.Host
	if host == "" {
		host = "github.com"
	}
	return "https://" + host
}

func repositoryFromPath(host, path string) (Repository, error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repository{}, fmt.Errorf("cannot identify GitHub owner/repository from %q", path)
	}
	return Repository{Host: host, Owner: parts[0], Name: parts[1]}, nil
}
