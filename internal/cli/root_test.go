package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/0stick/CodeWhy/internal/cli"
	"github.com/0stick/CodeWhy/internal/model"
)

func TestJSONErrorForInvalidTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.ExecuteArgs([]string{"--json", "invalid"}, &stdout, &stderr)
	if err == nil || !cli.ErrorWasRendered(err) {
		t.Fatalf("expected rendered command error, got %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var response model.ErrorResponse
	if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.SchemaVersion != model.SchemaVersion || response.Error.Code != "invalid_target" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestJSONErrorOutsideGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.ExecuteArgs([]string{"--json", "--offline", "main.go:1"}, &stdout, &stderr)
	if err == nil || !cli.ErrorWasRendered(err) {
		t.Fatalf("expected rendered command error, got %v", err)
	}

	var response model.ErrorResponse
	if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.Error.Code != "not_git_repository" {
		t.Fatalf("unexpected response: %#v", response)
	}
}
