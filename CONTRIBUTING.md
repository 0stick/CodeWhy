# Contributing to codewhy

Thanks for helping improve `codewhy`.

## Development Setup

Requirements:

- Go 1.24 or newer
- Git

```sh
git clone https://github.com/0stick/CodeWhy.git
cd codewhy
go test ./...
go vet ./...
go build ./cmd/codewhy
```

## Changes

- Keep Git execution argument-based; never construct shell command strings.
- Keep GitHub API tests local with `httptest`. Tests must not require network access.
- Preserve the JSON schema or deliberately version it.
- Add behavior-focused tests for fixes and features.
- Do not log credentials, authorization headers, or token values.

Before opening a pull request:

```sh
gofmt -w .
go vet ./...
go test ./...
```

Describe the user-visible behavior, tests performed, and any compatibility impact in the pull request.

## Reporting Issues

Use the relevant GitHub issue template. Include the `codewhy --version` output, operating system, Git version, command used, and sanitized error output. Never include access tokens or private repository content.
