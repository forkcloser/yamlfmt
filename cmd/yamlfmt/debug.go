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
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/forkcloser/yamlfmt"
)

// debugAll is the -debug code that admits every code.
const debugAll = "all"

// debugHandler is the slog handler the command installs: a record at Info or
// above is written as it comes; a debug record is written when its code
// attribute (yamlfmt.DebugCodeAttr) is among the -debug codes asked for, or
// "all" was. A line is the level, the message and the attributes as
// key=value, except that a multi-line value follows on the lines after, so a
// diff reads as a diff. Groups are not used by this program's records and
// are ignored.
type debugHandler struct {
	w     io.Writer
	codes map[string]bool
	attrs []slog.Attr
}

// newDebugHandler writes to w the records the -debug codes admit.
func newDebugHandler(w io.Writer, codes []string) *debugHandler {
	h := &debugHandler{w: w, codes: make(map[string]bool, len(codes))}
	for _, code := range codes {
		h.codes[code] = true
	}

	return h
}

// Enabled: Info and above always; Debug when any code was asked for.
func (h *debugHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo || len(h.codes) > 0
}

// Handle writes the record, or drops a debug record whose code was not asked for.
func (h *debugHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make([]slog.Attr, 0, len(h.attrs)+record.NumAttrs())
	attrs = append(attrs, h.attrs...)

	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)

		return true
	})

	if record.Level < slog.LevelInfo && !h.admits(attrs) {
		return nil
	}

	var line, tail strings.Builder

	line.WriteString(record.Level.String())
	line.WriteString(": ")
	line.WriteString(record.Message)

	for _, attr := range attrs {
		value := attr.Value.Resolve().String()
		if strings.Contains(value, "\n") {
			tail.WriteString("\n")
			tail.WriteString(value)

			continue
		}

		if strings.ContainsAny(value, " \t") || value == "" {
			value = fmt.Sprintf("%q", value)
		}

		fmt.Fprintf(&line, " %s=%s", attr.Key, value)
	}

	line.WriteString(tail.String())
	line.WriteString("\n")

	if _, err := io.WriteString(h.w, line.String()); err != nil {
		return fmt.Errorf("write the log line: %w", err)
	}

	return nil
}

// WithAttrs is the handler with attrs on every record.
func (h *debugHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &debugHandler{w: h.w, codes: h.codes, attrs: slices.Concat(h.attrs, attrs)}
}

// WithGroup is the handler itself: this program's records carry no groups.
func (*debugHandler) WithGroup(string) slog.Handler {
	panic("debugHandler: groups are not used by this program")
}

// admits reports whether a debug record's code was asked for.
func (h *debugHandler) admits(attrs []slog.Attr) bool {
	if h.codes[debugAll] {
		return true
	}

	for _, attr := range attrs {
		if attr.Key == yamlfmt.DebugCodeAttr && h.codes[attr.Value.String()] {
			return true
		}
	}

	return false
}
