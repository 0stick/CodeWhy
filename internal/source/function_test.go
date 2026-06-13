package source_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0stick/CodeWhy/internal/source"
)

func TestFindFunctionContainingLine(t *testing.T) {
	root := t.TempDir()
	code := "package sample\n\ntype Service struct{}\n\nfunc (s *Service) Run() {\n\tprintln(\"running\")\n}\n"
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	function, err := source.FindFunction(root, "service.go", 6)
	if err != nil {
		t.Fatal(err)
	}
	if function.Name != "Service.Run" || function.StartLine != 5 || function.EndLine != 7 {
		t.Fatalf("unexpected function: %#v", function)
	}
}

func TestFindFunctionRejectsUnsupportedLanguage(t *testing.T) {
	if _, err := source.FindFunction(t.TempDir(), "service.py", 1); err == nil {
		t.Fatal("expected unsupported language error")
	}
}
