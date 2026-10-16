// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yamlfmt_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/forkcloser/yamlfmt"
	"github.com/forkcloser/yamlfmt/internal/collections"
	"github.com/forkcloser/yamlfmt/internal/tempfile"
)

func TestFilepathCollectorExcludeMatching(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"a/b/x.yaml", "a/bc/y.yaml", "a/z.yaml"} {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte("a: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(root)

	withoutB := []string{"a/bc/y.yaml", "a/z.yaml"}

	for _, tc := range []struct {
		name    string
		exclude string
		want    []string
	}{
		{name: "directory", exclude: "a/b", want: withoutB},
		{name: "directory with a trailing separator", exclude: "a/b/", want: withoutB},
		{name: "directory written with ./", exclude: "./a/b", want: withoutB},
		{name: "absolute directory", exclude: filepath.Join(root, "a", "b"), want: withoutB},
		{name: "file written with ./", exclude: "./a/z.yaml", want: []string{"a/b/x.yaml", "a/bc/y.yaml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := &yamlfmt.FilepathCollector{
				Include:    []string{"."},
				Exclude:    []string{tc.exclude},
				Extensions: []string{"yaml"},
			}

			paths, err := collector.CollectPaths()
			if err != nil {
				t.Fatal(err)
			}

			got := make([]string, 0, len(paths))
			for _, path := range paths {
				got = append(got, filepath.ToSlash(path))
			}

			slices.Sort(got)

			if !slices.Equal(got, tc.want) {
				t.Fatalf("exclude %q: got %q, want %q", tc.exclude, got, tc.want)
			}
		})
	}
}

func TestFilepathCollector(t *testing.T) {
	testCaseTable{
		{
			name: "finds direct paths",
			files: []tempfile.Path{
				{FilePath: "x.yaml"},
				{FilePath: "y.yaml"},
				{FilePath: "z.yml"},
			},
			includePatterns: testPatterns{
				{pattern: "x.yaml"},
				{pattern: "y.yaml"},
				{pattern: "z.yml"},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml": {},
				"y.yaml": {},
				"z.yml":  {},
			},
		},
		{
			name: "finds all in directory one layer",
			files: []tempfile.Path{
				{FilePath: "a", IsDir: true},
				{FilePath: "a/x.yaml"},
				{FilePath: "a/y.yaml"},
				{FilePath: "a/z.yml"},
			},
			includePatterns: testPatterns{
				{pattern: "a"},
			},
			extensions: []string{
				"yaml",
				"yml",
			},
			expectedFiles: collections.Set[string]{
				"a/x.yaml": {},
				"a/y.yaml": {},
				"a/z.yml":  {},
			},
		},
		{
			name: "finds direct path to subdirectory",
			files: []tempfile.Path{
				{FilePath: "a", IsDir: true},
				{FilePath: "a/x.yaml"},
				{FilePath: "a/y.yaml"},
				{FilePath: "a/z.yml"},
			},
			includePatterns: testPatterns{
				{pattern: "a/x.yaml"},
				{pattern: "a/z.yml"},
			},
			expectedFiles: collections.Set[string]{
				"a/x.yaml": {},
				"a/z.yml":  {},
			},
		},
		{
			name: "finds all in layered directories",
			files: []tempfile.Path{
				{FilePath: "a", IsDir: true},
				{FilePath: "a/b", IsDir: true},
				{FilePath: "x.yml"},
				{FilePath: "y.yml"},
				{FilePath: "z.yaml"},
				{FilePath: "a/x.yaml"},
				{FilePath: "a/b/x.yaml"},
				{FilePath: "a/b/y.yml"},
			},
			includePatterns: testPatterns{
				{pattern: ""}, // with the test this functionally means the whole temp dir
			},
			extensions: []string{
				"yaml",
				"yml",
			},
			expectedFiles: collections.Set[string]{
				"x.yml":      {},
				"y.yml":      {},
				"z.yaml":     {},
				"a/x.yaml":   {},
				"a/b/x.yaml": {},
				"a/b/y.yml":  {},
			},
		},
		{
			name: "exclude files",
			files: []tempfile.Path{
				{FilePath: "a", IsDir: true},
				{FilePath: "a/b", IsDir: true},
				{FilePath: "x.yml"},
				{FilePath: "y.yml"},
				{FilePath: "z.yaml"},
				{FilePath: "a/x.yaml"},
				{FilePath: "a/b/x.yaml"},
				{FilePath: "a/b/y.yml"},
			},
			includePatterns: testPatterns{
				{pattern: ""}, // with the test this functionally means the whole temp dir
			},
			excludePatterns: testPatterns{
				{pattern: "x.yml"},
				{pattern: "a/x.yaml"},
			},
			extensions: []string{
				"yaml",
				"yml",
			},
			expectedFiles: collections.Set[string]{
				"y.yml":      {},
				"z.yaml":     {},
				"a/b/x.yaml": {},
				"a/b/y.yml":  {},
			},
		},
		{
			name:            "exclude directory",
			changeToTempDir: true,
			files: []tempfile.Path{
				{FilePath: "x.yml"},
				{FilePath: "y.yml"},
				{FilePath: "z.yaml"},

				{FilePath: "a", IsDir: true},
				{FilePath: "a/x.yaml"},

				{FilePath: "a/b", IsDir: true},
				{FilePath: "a/b/x.yaml"},
				{FilePath: "a/b/y.yml"},
			},
			includePatterns: testPatterns{
				{pattern: ""}, // with the test this functionally means the whole temp dir
			},
			excludePatterns: testPatterns{
				{pattern: "a/b"},
			},
			extensions: []string{
				"yaml",
				"yml",
			},
			expectedFiles: collections.Set[string]{
				"x.yml":    {},
				"y.yml":    {},
				"z.yaml":   {},
				"a/x.yaml": {},
			},
		},
		{
			name: "don't get files with wrong extension",
			files: []tempfile.Path{
				{FilePath: "x.yml"},
				{FilePath: "y.yaml"},
				{FilePath: "z.json"},
			},
			includePatterns: testPatterns{
				{pattern: ""}, // with the test this functionally means the whole temp dir
			},
			extensions: []string{
				"yaml",
				"yml",
			},
			expectedFiles: collections.Set[string]{
				"x.yml":  {},
				"y.yaml": {},
			},
		},
		{
			name: "multi-part extension",
			files: []tempfile.Path{
				{FilePath: "x.yaml"},
				{FilePath: "y.yaml.gotmpl"},
				{FilePath: "z.json"},
			},
			includePatterns: testPatterns{
				{pattern: ""}, // with the test this functionally means the whole temp dir
			},
			extensions: []string{
				"yaml",
				"yml",
				"yaml.gotmpl",
			},
			expectedFiles: collections.Set[string]{
				"x.yaml":        {},
				"y.yaml.gotmpl": {},
			},
		},
	}.runAll(t, useFilepathCollector)
}

func TestDoublestarCollectorBasic(t *testing.T) {
	testCaseTable{
		{
			name: "no excludes",
			files: []tempfile.Path{
				{FilePath: "x.yaml"},
				{FilePath: "y.yaml"},
				{FilePath: "z.yaml"},
			},
			includePatterns: testPatterns{
				{pattern: "**/*.yaml"},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml": {},
				"y.yaml": {},
				"z.yaml": {},
			},
		},
	}.runAll(t, useDoublestarCollector)
}

func TestDoublestarCollectorExcludeDirectory(t *testing.T) {
	testFiles := []tempfile.Path{
		{FilePath: "x.yaml"},

		{FilePath: "y", IsDir: true},
		{FilePath: "y/y.yaml"},

		{FilePath: "z", IsDir: true},
		{FilePath: "z/z.yaml"},
		{FilePath: "z/z1.yaml"},
		{FilePath: "z/z2.yaml"},
	}

	testCaseTable{
		{
			name:  "exclude_directory/start with doublestar",
			files: testFiles,
			includePatterns: testPatterns{
				{pattern: "**/*.yaml"},
			},
			excludePatterns: testPatterns{
				{pattern: "**/z/**/*.yaml", stayRelative: true},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml":   {},
				"y/y.yaml": {},
			},
		},
		{
			name:            "exclude_directory/relative include and exclude",
			changeToTempDir: true,
			files:           testFiles,
			includePatterns: testPatterns{
				{pattern: "**/*.yaml", stayRelative: true},
			},
			excludePatterns: testPatterns{
				{pattern: "z/**/*.yaml", stayRelative: true},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml":   {},
				"y/y.yaml": {},
			},
		},
		{
			name:  "exclude_directory/absolute include and exclude",
			files: testFiles,
			includePatterns: testPatterns{
				{pattern: "**/*.yaml"},
			},
			excludePatterns: testPatterns{
				{pattern: "z/**/*.yaml"},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml":   {},
				"y/y.yaml": {},
			},
		},
		{
			name:            "exclude_directory/absolute include relative exclude",
			skip:            true,
			changeToTempDir: true,
			files:           testFiles,
			includePatterns: testPatterns{
				{pattern: "**/*.yaml"},
			},
			excludePatterns: testPatterns{
				{pattern: "z/**/*.yaml", stayRelative: true},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml":   {},
				"y/y.yaml": {},
			},
		},
		{
			name:            "exclude_directory/relative include absolute exclude",
			skip:            true,
			changeToTempDir: true,
			files:           testFiles,
			includePatterns: testPatterns{
				{pattern: "**/*.yaml", stayRelative: true},
			},
			excludePatterns: testPatterns{
				{pattern: "z/**/*.yaml"},
			},
			expectedFiles: collections.Set[string]{
				"x.yaml":   {},
				"y/y.yaml": {},
			},
		},
	}.runAll(t, useDoublestarCollector)
}

type testPatterns []struct {
	pattern      string
	stayRelative bool
}

func (tps testPatterns) allPatterns(path string) []string {
	result := make([]string, len(tps))
	for i := 0; i < len(tps); i++ {
		if tps[i].stayRelative {
			result[i] = tps[i].pattern
		} else {
			result[i] = fmt.Sprintf("%s/%s", path, tps[i].pattern)
		}
	}

	return result
}

// In some test scenarios we want to ignore whether a pattern is marked stayRelative
// and always treat them as relative by formatting the base path on them.
func (tps testPatterns) allPatternsForceAbsolute(path string) []string {
	result := make([]string, len(tps))
	for i := 0; i < len(tps); i++ {
		result[i] = fmt.Sprintf("%s/%s", path, tps[i].pattern)
	}

	return result
}

type testCase struct {
	name            string
	skip            bool
	changeToTempDir bool
	files           []tempfile.Path
	includePatterns testPatterns
	extensions      []string
	excludePatterns testPatterns
	expectedFiles   collections.Set[string]
}

func (tc testCase) run(t *testing.T, makeCollector makeCollectorFunc) {
	t.Helper()

	t.Run(tc.name, func(t *testing.T) {
		if tc.skip {
			t.Skip()
		}

		tempPath := t.TempDir()

		if tc.changeToTempDir {
			t.Chdir(tempPath)
		}

		for _, file := range tc.files {
			file.BasePath = tempPath
			if err := file.Create(); err != nil {
				t.Fatalf("Failed to create file")
			}
		}

		collector := makeCollector(tc, tempPath)

		paths, err := collector.CollectPaths()
		if err != nil {
			t.Fatalf("Test case failed: %v", err)
		}

		filesToFormat := collections.Set[string]{}

		for _, path := range paths {
			formatPath := path
			if filepath.IsAbs(formatPath) {
				formatPath, err = filepath.Rel(tempPath, path)
				if err != nil {
					t.Fatalf("Path %s could not match to path %s", tempPath, path)
				}
			}

			filesToFormat.Add(filepath.ToSlash(formatPath))
		}

		if !filesToFormat.Equals(tc.expectedFiles) {
			t.Fatalf("Expected to receive paths %v\nbut got %v", tc.expectedFiles, filesToFormat)
		}
	})
}

type testCaseTable []testCase

func (tcs testCaseTable) runAll(t *testing.T, makeCollector makeCollectorFunc) {
	t.Helper()

	for _, tc := range tcs {
		tc.run(t, makeCollector)
	}
}

type makeCollectorFunc func(tc testCase, path string) yamlfmt.PathCollector

func useFilepathCollector(tc testCase, path string) yamlfmt.PathCollector {
	return &yamlfmt.FilepathCollector{
		Include:    tc.includePatterns.allPatterns(path),
		Exclude:    tc.excludePatterns.allPatterns(path),
		Extensions: tc.extensions,
	}
}

func useDoublestarCollector(tc testCase, path string) yamlfmt.PathCollector {
	var includePatterns []string
	if tc.changeToTempDir {
		includePatterns = tc.includePatterns.allPatterns(path)
	} else {
		// If we didn't change to temp dir, disallow relative paths so we don't pick up
		// something confusing from the main working directory.
		includePatterns = tc.includePatterns.allPatternsForceAbsolute(path)
	}

	return &yamlfmt.DoublestarCollector{
		Include: includePatterns,
		Exclude: tc.excludePatterns.allPatterns(path),
	}
}

func TestPatternFile(t *testing.T) {
	t.Parallel()

	makePatterns := func(patterns ...string) []byte {
		var b bytes.Buffer

		fmt.Fprintln(&b, "# Comment followed by empty line")
		fmt.Fprintln(&b)

		for _, p := range patterns {
			fmt.Fprintln(&b, p)
		}

		return b.Bytes()
	}

	cases := []struct {
		name      string
		patterns  []byte
		haveFiles []string
		wantFiles []string
	}{
		{
			name:      "yaml and yml files",
			patterns:  makePatterns("*.yaml", "*.yml"),
			haveFiles: []string{"x.yaml", "y.yml", "README.md"},
			wantFiles: []string{"x.yaml", "y.yml"},
		},
		{
			name:      "ignore pattern",
			patterns:  makePatterns("*.yaml", "*.yml", "!test_input.yaml"),
			haveFiles: []string{"x.yaml", "y.yml", "test_input.yaml"},
			wantFiles: []string{"x.yaml", "y.yml"},
		},
		{
			name:      "descent into directories",
			patterns:  makePatterns("*.yaml", "*.yml"),
			haveFiles: []string{"a/x.yaml", "b/y.yml"},
			wantFiles: []string{"a/x.yaml", "b/y.yml"},
		},
		{
			name:      "exclude directories",
			patterns:  makePatterns("*.yaml", "*.yml", "!a/"),
			haveFiles: []string{"a/x.yaml", "b/y.yml"},
			wantFiles: []string{"b/y.yml"},
		},
		{
			name:      "matches are rooted at the working directory",
			patterns:  makePatterns("*.yaml", "!/x.yaml"),
			haveFiles: []string{"x.yaml", "a/x.yaml"},
			wantFiles: []string{"a/x.yaml"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := make(fstest.MapFS)
			for _, f := range tc.haveFiles {
				fs[f] = &fstest.MapFile{
					Data: []byte("test"),
				}
			}

			patternFile := yamlfmt.NewPatternFileCollectorFS(bytes.NewReader(tc.patterns), fs)

			gotFiles, err := patternFile.CollectPaths()
			if err != nil {
				t.Fatal(err)
			}

			// Ignore the order of files in tc.wantFiles and gotFiles.
			opts := []cmp.Option{
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
			}

			if diff := cmp.Diff(tc.wantFiles, gotFiles, opts...); diff != "" {
				t.Errorf("PatternFile.CollectPaths() differs (-want/+got):\n%s", diff)
			}
		})
	}
}

// TestExcludeWithGitignoreFromSubdirectory: the gitignore is found at the
// repository's root and its patterns apply from there, wherever the command
// runs; without a gitignore nothing is excluded and nothing fails.
func TestExcludeWithGitignoreFromSubdirectory(t *testing.T) {
	repo := t.TempDir()

	for _, dir := range []string{".git", "sub"} {
		if err := os.Mkdir(filepath.Join(repo, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(filepath.Join(repo, "sub"))

	got, err := yamlfmt.ExcludeWithGitignore(".gitignore", []string{"x.yml", "y.yml"})
	if err != nil || len(got) != 2 {
		t.Fatalf("without a gitignore: %v, %v; want both paths and no error", got, err)
	}

	if err = os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("sub/x.yml\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err = yamlfmt.ExcludeWithGitignore(".gitignore", []string{"x.yml", "y.yml"})
	if err != nil || len(got) != 1 || got[0] != "y.yml" {
		t.Fatalf("from the subdirectory: %v, %v; want y.yml alone", got, err)
	}

	t.Chdir(repo)

	got, err = yamlfmt.ExcludeWithGitignore(".gitignore", []string{"sub/x.yml", "sub/y.yml"})
	if err != nil || len(got) != 1 || got[0] != "sub/y.yml" {
		t.Fatalf("from the root: %v, %v; want sub/y.yml alone", got, err)
	}

	if _, err = yamlfmt.ExcludeWithGitignore(filepath.Join(repo, "missing"), []string{"x.yml"}); err == nil {
		t.Fatal("an explicit gitignore path that does not exist was accepted")
	}
}
