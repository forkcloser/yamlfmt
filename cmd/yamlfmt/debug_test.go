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

package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/forkcloser/yamlfmt"
)

// TestDebugHandler: a debug record passes when its code was asked for, or
// "all" was, and is dropped otherwise; Info and above always pass; a
// multi-line value follows the line as it is.
func TestDebugHandler(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		codes []string
		want  []string
	}{
		{"no codes", nil, []string{"INFO: formatted n=3", "ERROR: failed err=\"no such file\""}},
		{"one code", []string{"paths"}, []string{"DEBUG: found paths code=paths paths=\"[a.yaml b.yaml]\"", "INFO: formatted n=3", "ERROR: failed err=\"no such file\""}},
		{"all", []string{"all"}, []string{
			"DEBUG: found paths code=paths paths=\"[a.yaml b.yaml]\"",
			"DEBUG: the following files were modified code=diffs\na.yaml:\n- a:  1  a: 1",
			"INFO: formatted n=3",
			"ERROR: failed err=\"no such file\"",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			logger := slog.New(newDebugHandler(&buf, tc.codes))
			logger.Debug("found paths",
				yamlfmt.DebugCodeAttr, yamlfmt.DebugCodePaths, "paths", []string{"a.yaml", "b.yaml"})
			logger.Debug("the following files were modified",
				yamlfmt.DebugCodeAttr, yamlfmt.DebugCodeDiffs, "diffs", "a.yaml:\n- a:  1  a: 1")
			logger.Info("formatted", "n", 3)
			logger.Error("failed", "err", "no such file")

			got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			want := strings.Split(strings.Join(tc.want, "\n"), "\n")

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("handler wrote:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// TestDebugHandlerWithAttrs: attrs given to WithAttrs lead every record of
// the derived logger, and admit a debug record by their code.
func TestDebugHandlerWithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(newDebugHandler(&buf, []string{"paths"})).With("file", "a.yaml")
	logger.Info("formatted", "n", 3)
	logger.With(yamlfmt.DebugCodeAttr, yamlfmt.DebugCodePaths).Debug("found")
	logger.With(yamlfmt.DebugCodeAttr, yamlfmt.DebugCodeDiffs).Debug("dropped")

	want := "INFO: formatted file=a.yaml n=3\nDEBUG: found file=a.yaml code=paths\n"
	if got := buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestDebugHandlerWithGroup: the program uses no groups, so WithGroup is
// refused rather than silently flattened.
func TestDebugHandlerWithGroup(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("WithGroup returned; want a panic")
		}
	}()

	newDebugHandler(&bytes.Buffer{}, nil).WithGroup("g")
}
