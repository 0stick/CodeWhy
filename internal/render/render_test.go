package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0stick/CodeWhy/internal/model"
	"github.com/0stick/CodeWhy/internal/render"
)

func TestJSONOutputHasStableShape(t *testing.T) {
	result := model.Result{
		SchemaVersion: model.SchemaVersion,
		Commit: model.Commit{
			SHA:     "abc",
			Author:  "Alice",
			Date:    "2024-08-12T10:00:00Z",
			Message: "Prevent duplicate refreshes",
			Files:   []string{},
		},
		Issues:     []model.Reference{},
		Reason:     "Prevent duplicate refreshes",
		Confidence: "medium",
		Warnings:   []string{},
	}
	var output bytes.Buffer
	if err := render.Result(&output, result, render.Options{JSON: true}); err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "commit", "issues", "reason", "confidence", "warnings"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing JSON key %q in %s", key, output.String())
		}
	}
	if strings.Contains(output.String(), `"issues": null`) || strings.Contains(output.String(), `"warnings": null`) {
		t.Fatalf("arrays must not be null: %s", output.String())
	}
}
