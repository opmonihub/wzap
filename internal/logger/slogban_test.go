package logger_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoSlogInProd is the grep gate of the canonical-zerolog-logger change:
// zerolog is the single logger in production code, so no non-test Go file
// may import log/slog anymore. Test helpers are exempt (they never ship),
// and so are vendored or generated trees.
func TestNoSlogInProd(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller cannot locate the test file")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))

	var offenders []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".worktrees", "vendor", "node_modules", ".output", ".nuxt", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for line := range strings.Lines(string(content)) {
			if strings.Contains(line, `"log/slog"`) {
				offenders = append(offenders, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("log/slog imported by %d prod file(s), want zerolog only:\n%s",
			len(offenders), strings.Join(offenders, "\n"))
	}
}
