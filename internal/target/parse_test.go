package target_test

import (
	"testing"

	"github.com/0stick/CodeWhy/internal/target"
)

func TestParseTarget(t *testing.T) {
	got, err := target.Parse("src/auth.go:42")
	if err != nil {
		t.Fatal(err)
	}
	if got.File != "src/auth.go" || got.Line != 42 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseWindowsTarget(t *testing.T) {
	got, err := target.Parse(`C:\work tree\src\auth.go:42`)
	if err != nil {
		t.Fatal(err)
	}
	if got.File != `C:\work tree\src\auth.go` || got.Line != 42 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseTargetRejectsInvalidLine(t *testing.T) {
	for _, value := range []string{"file.go", "file.go:0", "file.go:nope", ":12"} {
		if _, err := target.Parse(value); err == nil {
			t.Errorf("Parse(%q) succeeded", value)
		}
	}
}
