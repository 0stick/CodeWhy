package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0stick/CodeWhy/internal/model"
)

type Client struct {
	Dir string
}

type Blame struct {
	SHA        string
	SourceLine int
	SourceFile string
	Code       string
}

func (c Client) Root(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is required but was not found in PATH; install Git and try again")
	}
	out, err := c.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a Git repository: run codewhy from a cloned repository")
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}

func (c Client) BlameLine(ctx context.Context, file string, line int) (Blame, error) {
	lineArg := strconv.Itoa(line) + "," + strconv.Itoa(line)
	out, err := c.run(ctx, "blame", "--line-porcelain", "-w", "-M", "-C", "-L", lineArg, "--", file)
	if err != nil {
		return Blame{}, fmt.Errorf("cannot blame %s:%d: %w", file, line, err)
	}
	return parseBlame(out)
}

func (c Client) Commit(ctx context.Context, sha string) (model.Commit, error) {
	format := "%H%x00%an%x00%aI%x00%B"
	out, err := c.run(ctx, "show", "-s", "--format="+format, sha)
	if err != nil {
		return model.Commit{}, fmt.Errorf("cannot read commit %q: %w", sha, err)
	}
	parts := bytes.SplitN(out, []byte{0}, 4)
	if len(parts) != 4 {
		return model.Commit{}, fmt.Errorf("cannot parse metadata for commit %q", sha)
	}

	diff, err := c.run(ctx, "show", "--format=", "--no-ext-diff", "--unified=3", sha)
	if err != nil {
		return model.Commit{}, fmt.Errorf("cannot read diff for commit %q: %w", sha, err)
	}
	filesOut, err := c.run(ctx, "diff-tree", "--root", "--no-commit-id", "--name-only", "-r", "-z", sha)
	if err != nil {
		return model.Commit{}, fmt.Errorf("cannot list files for commit %q: %w", sha, err)
	}

	return model.Commit{
		SHA:     string(parts[0]),
		Author:  string(parts[1]),
		Date:    string(parts[2]),
		Message: strings.TrimSpace(string(parts[3])),
		Diff:    strings.TrimSpace(string(diff)),
		Files:   splitNUL(filesOut),
	}, nil
}

func (c Client) RemoteURL(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "remote", "get-url", name)
	if err != nil {
		return "", fmt.Errorf("cannot read Git remote %q: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func ReadContext(root, file string, line, radius int) ([]model.ContextLine, error) {
	if radius < 0 {
		return nil, fmt.Errorf("context must be zero or greater")
	}
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(file))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read target file %q: %w", file, err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if line > len(lines) {
		return nil, fmt.Errorf("line %d is beyond the end of %s (%d lines)", line, file, len(lines))
	}
	start := max(1, line-radius)
	end := min(len(lines), line+radius)
	result := make([]model.ContextLine, 0, end-start+1)
	for number := start; number <= end; number++ {
		result = append(result, model.ContextLine{
			Line:    number,
			Code:    lines[number-1],
			Current: number == line,
		})
	}
	return result, nil
}

func (c Client) run(ctx context.Context, args ...string) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s", message)
	}
	return out, nil
}

func parseBlame(data []byte) (Blame, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return Blame{}, fmt.Errorf("git blame returned no data")
	}
	header := strings.Fields(scanner.Text())
	if len(header) < 3 {
		return Blame{}, fmt.Errorf("unexpected git blame header")
	}
	sourceLine, err := strconv.Atoi(header[1])
	if err != nil {
		return Blame{}, fmt.Errorf("unexpected source line in git blame")
	}

	result := Blame{SHA: header[0], SourceLine: sourceLine}
	if strings.Trim(result.SHA, "0") == "" {
		return Blame{}, fmt.Errorf("the target line has uncommitted changes; commit or stash them before running codewhy")
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "filename ") {
			result.SourceFile = strings.TrimPrefix(line, "filename ")
		}
		if strings.HasPrefix(line, "\t") {
			result.Code = strings.TrimPrefix(line, "\t")
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return Blame{}, fmt.Errorf("read git blame output: %w", err)
	}
	return result, nil
}

func splitNUL(data []byte) []string {
	raw := bytes.Split(data, []byte{0})
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if len(item) > 0 {
			result = append(result, string(item))
		}
	}
	return result
}
