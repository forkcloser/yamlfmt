// Copyright 2026 Forkcloser
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/forkcloser/yamlfmt/engine"
	"github.com/forkcloser/yamlfmt/formatters/basic"
)

// lintEngine is an engine over the basic formatter with the output format.
func lintEngine(t *testing.T, format engine.EngineOutputFormat) *engine.ConsecutiveEngine {
	t.Helper()

	formatter, err := (&basic.BasicFormatterFactory{}).NewFormatter(nil)
	if err != nil {
		t.Fatal(err)
	}

	return &engine.ConsecutiveEngine{LineSepCharacter: "\n", Formatter: formatter, OutputFormat: format}
}

// writeFiles writes the named contents into dir and returns their paths.
func writeFiles(t *testing.T, dir string, files map[string]string) []string {
	t.Helper()

	paths := make([]string, 0, len(files))

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		paths = append(paths, path)
	}

	return paths
}

// TestLineOutputNamesChangedFilesOnly: the line format names the files whose
// formatting differs, in path order, and none that is already formatted.
func TestLineOutputNamesChangedFilesOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	paths := writeFiles(t, dir, map[string]string{
		"clean.yaml": "a: 1\n",
		"z.yaml":     "a:  1\n",
		"b.yaml":     "b:   2\n",
	})

	for range 5 {
		out, err := lintEngine(t, engine.EngineOutputSingeLine).Lint(paths)
		if err != nil || out == nil {
			t.Fatalf("%v, %v", out, err)
		}

		want := filepath.Join(dir, "b.yaml") + ": formatting difference found\n" +
			filepath.Join(dir, "z.yaml") + ": formatting difference found\n"
		if out.String() != want {
			t.Fatalf("line output:\n%s\nwant:\n%s", out.String(), want)
		}
	}
}

// The report's shape, as far as the test reads it.
type (
	lines struct {
		Begin int `json:"begin"`
		End   int `json:"end"`
	}
	location struct {
		Path  string `json:"path"`
		Lines lines  `json:"lines"`
	}
	finding struct {
		Fingerprint string   `json:"fingerprint"`
		Location    location `json:"location"`
	}
)

// TestGitlabOutputKeepsFindingsWhole: sorted by path, each finding keeps
// its own fingerprint and lines, whatever order the files came in.
func TestGitlabOutputKeepsFindingsWhole(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	paths := writeFiles(t, dir, map[string]string{
		"a.yaml": "a:  1\n",
		"b.yaml": "x: 1\ny: 2\nz:   3\n",
	})

	for range 5 {
		out, err := lintEngine(t, engine.EngineOutputGitlab).Lint(paths)
		if err != nil || out == nil {
			t.Fatalf("%v, %v", out, err)
		}

		var findings []finding
		if err := json.Unmarshal([]byte(out.String()), &findings); err != nil {
			t.Fatalf("%v in %s", err, out.String())
		}

		if len(findings) != 2 {
			t.Fatalf("%d findings, want 2", len(findings))
		}

		if findings[0].Location.Path != filepath.Join(dir, "a.yaml") || findings[0].Location.Lines.Begin != 1 {
			t.Fatalf("first finding: %+v; want a.yaml at line 1", findings[0])
		}

		if findings[1].Location.Path != filepath.Join(dir, "b.yaml") || findings[1].Location.Lines.Begin != 3 {
			t.Fatalf("second finding: %+v; want b.yaml at line 3", findings[1])
		}

		if findings[0].Fingerprint == findings[1].Fingerprint {
			t.Fatal("the two findings share a fingerprint")
		}
	}
}
