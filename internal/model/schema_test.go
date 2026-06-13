package model_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedJSONSchemaIsValidJSON(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "schema", "codewhy-result.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["$schema"] == nil || schema["$defs"] == nil {
		t.Fatalf("schema is missing required metadata: %#v", schema)
	}
}
