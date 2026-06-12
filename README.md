# codewhy

[English](README.md) | [简体中文](README.zh-CN.md)

`codewhy` is a code archaeology CLI that traces a line back through Git and GitHub to answer: **Why does this code exist?**

```text
$ codewhy src/auth.ts:42
Code: src/auth.ts:42
Target: if refreshing { return existingRequest }
Introduced: 4f28c31 on 2024-08-12 by Alice
Commit: Prevent duplicate token refresh requests
Commit URL: https://github.com/acme/app/commit/4f28c31...
Pull Request: #418 Prevent concurrent token refresh
PR URL: https://github.com/acme/app/pull/418
Related Issue: #391 Refresh races invalidate tokens
Issue URL: https://github.com/acme/app/issues/391
Reason: Prevent concurrent token refresh. Concurrent requests could invalidate each other's tokens.
Confidence: high
```

## Installation

Download a binary from [GitHub Releases](https://github.com/0stick/CodeWhy/releases), or build from source:

```sh
go install github.com/0stick/CodeWhy/cmd/codewhy@latest
```

## Usage

Run `codewhy` anywhere inside a Git repository:

```sh
codewhy src/auth.ts:42
codewhy explain "src/path with spaces/auth.ts:42"
codewhy commit 4f28c31
```

Windows paths are supported:

```powershell
codewhy 'C:\work\app\src\auth.ts:42'
```

Options:

```text
--json              Output machine-readable JSON
--no-color          Disable terminal colors
--offline           Only read local Git information
--remote <name>     Select a Git remote (default: origin)
--context <number>  Show nearby source lines
--verbose           Show analysis progress on stderr
```

Run `codewhy --help` for the complete command reference.

## JSON Output

The JSON schema is versioned and uses empty arrays rather than `null` for collection fields:

```json
{
  "schema_version": "1",
  "target": {
    "file": "src/auth.ts",
    "line": 42,
    "source_file": "src/auth.ts",
    "source_line": 42,
    "code": "if (refreshing) return existingRequest"
  },
  "commit": {
    "sha": "4f28c31...",
    "author": "Alice",
    "date": "2024-08-12T10:30:00Z",
    "message": "Prevent duplicate token refresh requests",
    "diff": "...",
    "files": ["src/auth.ts"],
    "url": "https://github.com/acme/app/commit/4f28c31..."
  },
  "pull_request": {
    "number": 418,
    "title": "Prevent concurrent token refresh",
    "body": "Concurrent requests could invalidate each other's tokens.",
    "url": "https://github.com/acme/app/pull/418",
    "state": "closed"
  },
  "issues": [],
  "reason": "Prevent concurrent token refresh. Concurrent requests could invalidate each other's tokens.",
  "confidence": "high",
  "warnings": []
}
```

## GitHub Token

Authentication is optional for public repositories, but avoids GitHub's low anonymous rate limit. `codewhy` checks these sources in order:

1. `CODEWHY_GITHUB_TOKEN`
2. `GITHUB_TOKEN`
3. `gh auth token`, when the GitHub CLI is installed
4. Anonymous GitHub API access

Tokens are sent only in the GitHub authorization header and are never printed or logged.

## How It Works

1. Verifies the current directory is inside a Git repository.
2. Runs `git blame --line-porcelain -w -M -C` for the selected line.
3. Reads commit metadata, changed files, and diff using stable Git formats.
4. Parses the selected remote to identify a GitHub repository.
5. Queries GitHub for an associated pull request and referenced issues.
6. Produces a deterministic reason from PR, issue, or commit documentation.

Confidence means:

- `high`: a PR or issue provides documentation.
- `medium`: a clear commit message is the best available source.
- `low`: no clear documented reason was found.

`codewhy` does not present diff-based guesses as facts.

## Privacy

Local source code, diffs, and commit messages stay on your machine. Network requests contain repository identifiers, commit SHAs, and issue or PR numbers required by the GitHub API. `codewhy` does not call an AI API and does not send source code to GitHub beyond normal API identifiers.

Use `--offline` to disable all remote inspection and GitHub requests.

## Current Limitations

- Tracks the blame-selected origin, not the complete history of a line.
- Supports GitHub remotes only; local Git results still work for other hosts.
- PR association depends on GitHub's commit-to-PR API.
- Reason summaries are deterministic excerpts, not semantic AI summaries.
- Uncommitted target lines must be committed or stashed before analysis.

## Roadmap

- Trace complete line history, not only the latest relevant change
- Function and code-block tracking
- GitLab support
- PR review comment analysis
- ADR and documentation search
- Optional local or remote LLM summaries
- VS Code extension
- MCP Server

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, testing, and pull request guidance.

## License

MIT. See [LICENSE](LICENSE).
