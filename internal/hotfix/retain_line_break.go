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

// scalarKind is the kind of multi-line scalar the scanner may be inside.
type scalarKind int

const (
	// block is a | or > scalar: content is every line indented past parent.
	block scalarKind = iota
	// quoted is a "…" or '…' scalar open past its line: content is every
	// line up to the one that closes it, whatever its indent.
	quoted
	// plain is an unquoted scalar whose lines fold: content is every line
	// indented past parent that is not a comment.
	plain
)

// scalar is a multi-line scalar whose content the scanner is inside. A
// blank line inside one is the value's, not a line break to retain: a
// placeholder is a comment to YAML, and a comment folds into a folded or
// quoted scalar's value and ends a plain one. So none goes in, and the blank
// lines there pass through as the content they are.
type scalar struct {
	kind   scalarKind
	parent int  // the indent block and plain content must exceed
	keep   bool // a block with keep chomping: trailing blank lines are content
	quote  byte // the quote that closes a quoted scalar
}

// blockIndicator reports whether line opens a block scalar: its last token,
// before any comment, is | or > with chomping and indentation indicators.
// parent is the indent the scalar's content must exceed: the key's column,
// or the last dash's for a sequence entry that is the scalar itself.
func blockIndicator(line string) (parent int, keep, ok bool) {
	indent := lineIndent(line)

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

// opens reports the multi-line scalar line opens, if any: a block scalar by
// its indicator, a quoted scalar whose closing quote is not on the line, or
// a plain scalar, which may continue on more-indented lines. owner is the
// column of the key or dash whose value the line carries when it is a
// value alone on the line after a "key:" or a "-", or -1.
func opens(line string, owner int) (*scalar, bool) {
	if parent, keep, ok := blockIndicator(line); ok {
		return &scalar{kind: block, parent: parent, keep: keep}, true
	}

	col, value, bare := valueOf(line)
	if value == "" {
		return nil, false
	}

	if bare && owner >= 0 {
		col = owner
	}

	switch value[0] {
	case '"', '\'':
		if closesQuote(value[1:], value[0]) {
			return nil, false
		}

		return &scalar{kind: quoted, quote: value[0]}, true
	case '[', '{', '#', '|', '>', '*':
		return nil, false
	default:
		return &scalar{kind: plain, parent: col}, true
	}
}

// valueOf is the value part of a line that is not blank, with the column a
// continuation must be indented past: the key's for "key: value", the last
// dash's for "- value", the line's own for a value alone, which bare
// reports. A "key:" or a "-" with nothing after it has no value, and the
// column is then the key's or the dash's, for the value on the next line.
// An anchor or tag and a trailing comment are stripped from the value.
func valueOf(line string) (col int, value string, bare bool) {
	indent := lineIndent(line)
	rest := strings.TrimRight(line[indent:], " \t")
	col = indent
	keyCol := indent
	bare = true

	for rest == "-" || strings.HasPrefix(rest, "- ") {
		if rest == "-" {
			return keyCol, "", false
		}

		bare = false
		col = keyCol
		rest = rest[2:]
		keyCol += 2
	}

	// a quoted key: skip it
	if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
		if end := quoteEnd(rest[1:], rest[0]); end >= 0 {
			rest = rest[1+end+1:]
		}
	}

	switch i := strings.Index(rest, ": "); {
	case i >= 0:
		return keyCol, bareValue(rest[i+2:]), false
	case strings.HasSuffix(rest, ":"):
		return keyCol, "", false
	default:
		return col, bareValue(rest), bare
	}
}

// bareValue is value with its anchor or tag and a trailing comment
// stripped; a quoted value keeps its comment, which may be inside it.
func bareValue(value string) string {
	value = strings.TrimLeft(value, " \t")

	for value != "" && (value[0] == '&' || value[0] == '!') {
		i := strings.IndexAny(value, " \t")
		if i < 0 {
			return ""
		}

		value = strings.TrimLeft(value[i:], " \t")
	}

	if value != "" && value[0] != '"' && value[0] != '\'' {
		if i := strings.Index(value, " #"); i >= 0 {
			value = value[:i]
		}
	}

	return strings.TrimRight(value, " \t")
}

// quoteEnd is the index in text of the quote that closes a scalar opened
// with quote, or -1: a backslash escapes the next character in a
// double-quoted scalar, and a doubled quote stands for one in a
// single-quoted one.
func quoteEnd(text string, quote byte) int {
	for i := 0; i < len(text); i++ {
		switch {
		case quote == '"' && text[i] == '\\':
			i++
		case text[i] != quote:
		case quote == '\'' && i+1 < len(text) && text[i+1] == '\'':
			i++
		default:
			return i
		}
	}

	return -1
}

// closesQuote reports whether text, the rest of a line past an opening
// quote, closes the scalar.
func closesQuote(text string, quote byte) bool {
	return quoteEnd(text, quote) >= 0
}

// contains reports whether line, which is not blank, is the scalar's
// content: past parent for a block, past parent and not a comment for a
// plain scalar, any line for a quoted one until the one that closes it.
func (s *scalar) contains(line string) bool {
	switch s.kind {
	case quoted:
		return true
	case plain:
		indent := lineIndent(line)

		return indent > s.parent && line[indent] != '#'
	case block:
		return lineIndent(line) > s.parent
	default:
		return false
	}
}

// closes reports whether line, content of a quoted scalar, closes it.
func (s *scalar) closes(line string) bool {
	return s.kind == quoted && closesQuote(line, s.quote)
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
	region       *scalar
	pending      []string
	// owner is the column of the last "key:" or "-" with nothing after
	// it, whose value the next line that is not blank may carry, or -1.
	owner int
}

// blank reports whether line is a line break or an empty space line.
func blank(line string) bool { return strings.TrimSpace(line) == "" }

// inside reports whether the pending blank lines are a scalar's content:
// the scalar goes on past them with next, or keeps its trailing blank
// lines.
func (s *breakScanner) inside(next string, atEnd bool) bool {
	return s.region != nil && ((!atEnd && s.region.contains(next)) || s.region.keep)
}

// line writes a line that is not blank: the scalar's content when it is,
// the end of the scalar otherwise, and then the start of the scalar the
// line opens, if any.
func (s *breakScanner) line(txt string) {
	s.buf.WriteString(txt)
	s.buf.WriteString(s.newline)

	if s.region != nil {
		switch {
		case !s.region.contains(txt):
			s.region = nil
		case s.region.closes(txt):
			s.region = nil

			return
		default:
			return
		}
	}

	// A comment between a key and its value leaves the key's column to the
	// value.
	if strings.HasPrefix(strings.TrimLeft(txt, " "), "#") {
		return
	}

	owner := s.owner
	s.owner = -1

	if region, ok := opens(txt, owner); ok {
		s.region = region

		return
	}

	if col, value, _ := valueOf(txt); value == "" {
		s.owner = col
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
// inside a scalar they are content; after its last line they are breaks,
// placed at the parent's indent so the placeholder ends the scalar rather
// than joining it; elsewhere breaks at the padding.
func (s *breakScanner) replace(next string, atEnd bool) {
	switch {
	case s.inside(next, atEnd):
		s.verbatim()
	case s.region != nil:
		s.placeholders(strings.Repeat(" ", s.region.parent))
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
		breaks := &breakScanner{newline: newlineStr, chomp: chomp, owner: -1}
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
		breaks := &breakScanner{newline: newlineStr, owner: -1}
		scanner := linescan.NewScanner(content)

		for scanner.Scan() {
			txt := scanner.Text()
			if blank(txt) {
				breaks.pending = append(breaks.pending, txt)

				continue
			}

			breaks.restore(txt, false)

			if strings.HasPrefix(strings.TrimLeft(txt, " "), lineBreakPlaceholder) {
				if breaks.region != nil && !breaks.region.contains(txt) {
					breaks.region = nil
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
