package engine_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forkcloser/yamlfmt/engine"
	"github.com/forkcloser/yamlfmt/formatters/basic"
)

func TestLintReportsAPathGivenTwice(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "x.yaml")
	if err := os.WriteFile(path, []byte("a: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	formatter, err := (&basic.BasicFormatterFactory{}).NewFormatter(nil)
	if err != nil {
		t.Fatal(err)
	}

	eng := &engine.ConsecutiveEngine{LineSepCharacter: "\n", Formatter: formatter}

	_, err = eng.Lint([]string{path})
	if err != nil {
		t.Fatalf("once: %v", err)
	}

	_, err = eng.Lint([]string{path, path})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("twice: want the duplicate reported, got %v", err)
	}
}
