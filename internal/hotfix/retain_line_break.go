// Copyright 2022 Google LLC
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

// The features in this file are to retain line breaks.
// The basic idea is to insert/remove placeholder comments in the yaml document before and after the format process.

package hotfix

import (
	"bytes"
	"context"
	"strings"

	"github.com/forkcloser/yamlfmt"
	"github.com/forkcloser/yamlfmt/internal/linescan"
)

const lineBreakPlaceholder = "#magic___^_^___line"

type paddinger struct {
	strings.Builder
}

func (p *paddinger) adjust(txt string) {
	var indentSize int
	for i := 0; i < len(txt) && txt[i] == ' '; i++ { // yaml only allows space to indent.
		indentSize++
	}
	// Grows if the given size is larger than us and always return the max padding.
	for diff := indentSize - p.Len(); diff > 0; diff-- {
		p.WriteByte(' ')
	}
}

func MakeFeatureRetainLineBreak(linebreakStr string, chomp bool) yamlfmt.Feature {
	return yamlfmt.Feature{
		Name:         "Retain Line Breaks",
		BeforeAction: replaceLineBreakFeature(linebreakStr, chomp),
		AfterAction:  restoreLineBreakFeature(linebreakStr),
	}
}

// blockScalar is a block scalar (| or >) whose content the scanner is
// inside: every line indented past parent, every blank line before the next
// such line, and, with keep chomping, the blank lines up to the dedent. A
// placeholder is a comment to YAML, and inside a folded scalar it folds into
// the value; so none goes in, and the blank lines there pass through as the
// content they are.
type blockScalar struct {
	parent int
	keep   bool
}

// blockIndicator reports whether line opens a block scalar: its last token,
// before any comment, is | or > with chomping and indentation indicators.
// parent is the indent the scalar's content must exceed: the key's column,
// or the last dash's for a sequence entry that is the scalar itself.
func blockIndicator(line string) (parent int, keep, ok bool) {
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}

	rest := line[indent:]
	if i := strings.Index(rest, " #"); i >= 0 {
		rest = rest[:i]
	}

	rest = strings.TrimRight(rest, " \t")
	if rest == "" {
		return 0, false, false
	}

	token := rest[strings.LastIndexAny(rest, " \t")+1:]
	if !isBlockIndicator(token) {
		return 0, false, false
	}

	col := indent

	for strings.HasPrefix(rest, "- ") {
		if isBlockIndicator(strings.TrimSpace(rest[2:])) {
			return col, strings.Contains(token, "+"), true
		}

		rest = rest[2:]
		col += 2
	}

	return col, strings.Contains(token, "+"), true
}

// isBlockIndicator is | or > with up to two indicator characters: a
// chomping sign and an indentation digit, in either order.
func isBlockIndicator(token string) bool {
	if token == "" || (token[0] != '|' && token[0] != '>') || len(token) > 3 {
		return false
	}

	for _, c := range token[1:] {
		if c != '+' && c != '-' && (c < '0' || c > '9') {
			return false
		}
	}

	return true
}

// lineIndent is the number of leading spaces.
func lineIndent(line string) int {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}

	return n
}

// blockEnds reports whether line, which is not blank, ends the block: it is
// not indented past the block's parent.
func (b *blockScalar) blockEnds(line string) bool {
	return b != nil && lineIndent(line) <= b.parent
}

// breakScanner walks a document line by line, holding back blank lines
// until the next line that is not blank says what they are: a block
// scalar's content, or line breaks to replace with placeholders (before the
// format) and restore from them (after).
type breakScanner struct {
	buf     bytes.Buffer
	newline string
	chomp   bool

	inLineBreaks bool
	padding      paddinger
	block        *blockScalar
	pending      []string
}

// blank reports whether line is a line break or an empty space line.
func blank(line string) bool { return strings.TrimSpace(line) == "" }

// inside reports whether the pending blank lines are a block scalar's
// content: the block goes on past them with next, or keeps its trailing
// blank lines.
func (s *breakScanner) inside(next string, atEnd bool) bool {
	return s.block != nil && ((!atEnd && !s.block.blockEnds(next)) || s.block.keep)
}

// line writes a line that is not blank, ending the block it dedents from
// and opening the block it starts.
func (s *breakScanner) line(txt string) {
	if s.block.blockEnds(txt) {
		s.block = nil
	}

	s.buf.WriteString(txt)
	s.buf.WriteString(s.newline)

	if s.block == nil {
		if parent, keep, ok := blockIndicator(txt); ok {
			s.block = &blockScalar{parent: parent, keep: keep}
		}
	}
}

// verbatim writes the pending blank lines as they were.
func (s *breakScanner) verbatim() {
	for _, line := range s.pending {
		s.buf.WriteString(line)
		s.buf.WriteString(s.newline)
	}

	s.pending = s.pending[:0]
}

// placeholders writes a placeholder for the pending blank lines, one in all
// under chomp, at indent.
func (s *breakScanner) placeholders(indent string) {
	for range s.pending {
		if s.chomp && s.inLineBreaks {
			continue
		}

		s.buf.WriteString(indent)
		s.buf.WriteString(lineBreakPlaceholder)
		s.buf.WriteString(s.newline)

		s.inLineBreaks = true
	}

	s.pending = s.pending[:0]
}

// replace places the pending blank lines before next, or at the end:
// inside a block they are content; after its last line they are breaks,
// placed at the parent's indent so the placeholder ends the block rather
// than joining it; elsewhere breaks at the padding.
func (s *breakScanner) replace(next string, atEnd bool) {
	switch {
	case s.inside(next, atEnd):
		s.verbatim()
	case s.block != nil:
		s.placeholders(strings.Repeat(" ", s.block.parent))
	default:
		s.placeholders(s.padding.String())
	}
}

// restore keeps the pending blank lines when they are a block's content
// and drops them otherwise: the basic yaml lib inserts a newline when a
// comment (a placeholder or the user's) is followed by a `---`, so outside a
// block an empty line is only ever ours, restored from a placeholder.
func (s *breakScanner) restore(next string, atEnd bool) {
	if s.inside(next, atEnd) {
		s.verbatim()
	}

	s.pending = s.pending[:0]
}

func replaceLineBreakFeature(newlineStr string, chomp bool) yamlfmt.FeatureFunc {
	return func(_ context.Context, content []byte) (context.Context, []byte, error) {
		breaks := &breakScanner{newline: newlineStr, chomp: chomp}
		scanner := linescan.NewScanner(content)

		for scanner.Scan() {
			txt := scanner.Text()
			breaks.padding.adjust(txt)

			if blank(txt) {
				breaks.pending = append(breaks.pending, txt)

				continue
			}

			breaks.replace(txt, false)
			breaks.line(txt)

			breaks.inLineBreaks = false
		}

		breaks.replace("", true)

		return nil, breaks.buf.Bytes(), scanner.Err()
	}
}

func restoreLineBreakFeature(newlineStr string) yamlfmt.FeatureFunc {
	return func(_ context.Context, content []byte) (context.Context, []byte, error) {
		breaks := &breakScanner{newline: newlineStr}
		scanner := linescan.NewScanner(content)

		for scanner.Scan() {
			txt := scanner.Text()
			if blank(txt) {
				breaks.pending = append(breaks.pending, txt)

				continue
			}

			breaks.restore(txt, false)

			if strings.HasPrefix(strings.TrimLeft(txt, " "), lineBreakPlaceholder) {
				if breaks.block.blockEnds(txt) {
					breaks.block = nil
				}

				breaks.buf.WriteString(newlineStr)

				continue
			}

			breaks.line(txt)
		}

		breaks.restore("", true)

		return nil, breaks.buf.Bytes(), scanner.Err()
	}
}
