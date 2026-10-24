package yamlfmt_test

import (
	"path/filepath"
	"testing"

	"github.com/forkcloser/yamlfmt"
)

// TestPathInPatternForm: a pure function, held whole. A relative pattern
// meets the path relative to the working directory, a path under it; one
// outside stays as given, and a sibling whose name starts with two dots is
// under it, not outside; the working directory itself is "."; an absolute
// pattern meets the absolute path, a relative one joined to the directory.
func TestPathInPatternForm(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	outside := filepath.Join(filepath.Dir(cwd), "other", "z", "a.yaml")
	sibling := filepath.Join(cwd, "..x", "a.yaml")

	for _, tc := range []struct{ name, pattern, path, want string }{
		{"under the directory, relative pattern", "z/**/*.yaml", filepath.Join(cwd, "z", "a.yaml"), filepath.Join("z", "a.yaml")},
		{"outside the directory, relative pattern", "z/**/*.yaml", outside, outside},
		{"a two-dot sibling, relative pattern", "..x/**", sibling, filepath.Join("..x", "a.yaml")},
		{"the directory itself, relative pattern", "**", cwd, "."},
		{"relative path, absolute pattern", filepath.Join(cwd, "**"), filepath.Join("z", "a.yaml"), filepath.Join(cwd, "z", "a.yaml")},
		{"absolute path, absolute pattern", filepath.Join(cwd, "**"), filepath.Join(cwd, "z", "a.yaml"), filepath.Join(cwd, "z", "a.yaml")},
		{"relative path, relative pattern", "z/**", filepath.Join("z", "a.yaml"), filepath.Join("z", "a.yaml")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := yamlfmt.PathForPattern(cwd, tc.pattern, tc.path); got != tc.want {
				t.Errorf("PathForPattern(%q, %q, %q) = %q, want %q", cwd, tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}
