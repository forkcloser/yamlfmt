// Copyright 2026 Forkcloser. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
