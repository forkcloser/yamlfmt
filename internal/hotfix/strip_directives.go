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

package hotfix

import (
	"context"
	"strings"

	"github.com/forkcloser/yamlfmt"
	"github.com/forkcloser/yamlfmt/internal/linescan"
)

type directiveKey string

var contextDirectivesKey directiveKey = "directives"

type Directive struct {
	line    int
	content string
	// markerAfter is set on the last directive of a run that the input
	// followed with a document start marker, which the formatter leaves out
	// of a first document unless include_document_start is set.
	markerAfter bool
	// endBefore is set on the first directive of a run that the input
	// preceded with a document end marker, which the formatter writes as
	// the next document's start marker instead.
	endBefore bool
}

func ContextWithDirectives(ctx context.Context, directives []Directive) context.Context {
	return context.WithValue(ctx, contextDirectivesKey, directives)
}

func DirectivesFromContext(ctx context.Context) []Directive {
	return ctx.Value(contextDirectivesKey).([]Directive)
}

func MakeFeatureStripDirectives(lineSepChar string) yamlfmt.Feature {
	return yamlfmt.Feature{
		Name:         "Strip Directives",
		BeforeAction: stripDirectivesFeature(lineSepChar),
		AfterAction:  restoreDirectivesFeature(lineSepChar),
	}
}

func stripDirectivesFeature(lineSepChar string) yamlfmt.FeatureFunc {
	return func(ctx context.Context, content []byte) (context.Context, []byte, error) {
		directives := []Directive{}
		scanner := linescan.NewScanner(content)

		var result strings.Builder

		currLine := 1
		previous := ""

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "%") {
				directives = append(directives, Directive{
					line:      currLine,
					content:   line,
					endBefore: isDocumentEnd(previous),
				})
			} else {
				last := len(directives) - 1
				if last >= 0 && directives[last].line == currLine-1 && isDocumentStart(line) {
					directives[last].markerAfter = true
				}

				result.WriteString(line + lineSepChar)
			}

			previous = line
			currLine++
		}

		return ContextWithDirectives(ctx, directives), []byte(result.String()), nil
	}
}

func restoreDirectivesFeature(lineSepChar string) yamlfmt.FeatureFunc {
	return func(ctx context.Context, content []byte) (context.Context, []byte, error) {
		restore := &restorer{directives: DirectivesFromContext(ctx), line: 1}
		scanner := linescan.NewScanner(content)

		for scanner.Scan() {
			// A directive without its document start marker is not YAML.
			if restore.placeDue() && !isDocumentStart(scanner.Text()) {
				restore.write("---")
			}

			restore.write(scanner.Text())
		}

		// A directive can also be the file's last line.
		restore.placeDue()

		var result strings.Builder
		for _, line := range restore.lines {
			result.WriteString(line + lineSepChar)
		}

		return ctx, []byte(result.String()), nil
	}
}

// restorer puts the stripped directives back among the formatted lines, each
// at the input line it came from.
type restorer struct {
	directives []Directive
	next       int // the next directive to place
	lines      []string
	line       int // the input line the next line written stands for
}

// placeDue writes the run of directives due at the current line, if any, and
// reports whether the input followed the run with a document start marker.
func (r *restorer) placeDue() bool {
	// A directive between documents follows the end of the one before: the
	// formatter wrote that end as the next document's start.
	if r.due() && r.directives[r.next].endBefore && len(r.lines) > 0 && r.lines[len(r.lines)-1] == "---" {
		r.lines[len(r.lines)-1] = "..."
	}

	markerAfter := false

	// Consecutive directives are due one after the other at the same point.
	for r.due() {
		markerAfter = r.directives[r.next].markerAfter
		r.write(r.directives[r.next].content)
		r.next++
	}

	return markerAfter
}

func (r *restorer) due() bool {
	return r.next < len(r.directives) && r.directives[r.next].line == r.line
}

func (r *restorer) write(line string) {
	r.lines = append(r.lines, line)
	r.line++
}

// isDocumentEnd reports whether line closes a document with its marker.
func isDocumentEnd(line string) bool {
	return line == "..." || strings.HasPrefix(line, "... ") || strings.HasPrefix(line, "...\t")
}

// isDocumentStart reports whether line opens a document with its marker.
func isDocumentStart(line string) bool {
	return line == "---" || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "---\t")
}
