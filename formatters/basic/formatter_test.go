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

package basic_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/forkcloser/yamlfmt/formatters/basic"
	"github.com/forkcloser/yamlfmt/formatters/basic/features"
	"github.com/forkcloser/yamlfmt/internal/assert"
	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

// Quoted, so a stray \r or trailing space shows in the failure.
const outputMismatch = "formatted output differs.\nexpected: %q\n     got: %q"

var factory = basic.BasicFormatterFactory{}

func TestFormatter(t *testing.T) {
	testCases := []struct {
		name                     string
		config                   map[string]any
		badConfigErr             error
		formatErr                bool
		input                    string
		expect                   string
		skipLineEndNormalization bool
	}{
		{
			name:  "retains comments",
			input: `x: "y" # foo comment`,
		},
		{
			name: "parses multiple documents",
			input: `a:
---
b:`,
		},
		{
			name: "include document start",
			config: map[string]any{
				"include_document_start": true,
			},
			input: `a:`,
			expect: `---
a:`,
		},
		{
			name: "crlf line ending",
			config: map[string]any{
				"line_ending": "crlf",
			},
			input:  "a:\nb:\nc:\n",
			expect: "a:\r\nb:\r\nc:\r\n",

			skipLineEndNormalization: true,
		},
		{
			name: "lf line ending",
			config: map[string]any{
				"line_ending": "lf",
			},
			input:  "a:\r\nb:\r\nc:\r\n",
			expect: "a:\nb:\nc:\n",
		},
		{
			name:  "emoji support",
			input: `a: 😊`,
		},
		{
			name: "scan folded as literal",
			config: map[string]any{
				"scan_folded_as_literal": true,
				"retain_line_breaks":     true,
			},
			input: `a: >
  multiline
  folded
  scalar`,
			// The input ends without a line break, so the value has none:
			// strip chomping, where clip would add one.
			expect: `a: >-
  multiline
  folded
  scalar`,
		},
		{
			name: "indentless arrays",
			config: map[string]any{
				"indentless_arrays": true,
			},
			input: `a:
  - 1
  - 2`,
			expect: `a:
- 1
- 2`,
		},
		{
			name: "drop merge tag",
			config: map[string]any{
				"drop_merge_tag": true,
			},
			input: `a: &a
  b:
    <<: *a`,
		},
		{
			name: "pad line comments",
			config: map[string]any{
				"pad_line_comments": 2,
			},
			input:  `a: 1 # line comment`,
			expect: `a: 1  # line comment`,
		},
		{
			name: "preserve comment indents keeps manual column alignment",
			config: map[string]any{
				"preserve_comment_indents": true,
			},
			input: `steps:
  - a    # first
  - bb   # second`,
			expect: `steps:
  - a    # first
  - bb   # second`,
		},
		{
			name: "without preserve comment indents alignment collapses",
			input: `steps:
  - a    # first
  - bb   # second`,
			expect: `steps:
  - a # first
  - bb # second`,
		},
		{
			name: "preserve comment indents falls back when content overflows original column",
			config: map[string]any{
				"indent":                   4,
				"preserve_comment_indents": true,
			},
			input: `a:
  b: c # x`,
			expect: `a:
    b: c # x`,
		},
		{
			// Content grows (indent 2 -> 4) but the '#' column (11) is still
			// reachable: the column is held and the gap SHRINKS from 4 to 2.
			name: "preserve comment indents holds column when content grows within reach",
			config: map[string]any{
				"indent":                   4,
				"preserve_comment_indents": true,
			},
			input: `a:
  bb: c    # x`,
			expect: `a:
    bb: c  # x`,
		},
		{
			// Content shrinks (indent 4 -> 2): the column (11) is held and the
			// gap WIDENS from 2 to 4.
			name: "preserve comment indents holds column when content shrinks",
			config: map[string]any{
				"indent":                   2,
				"preserve_comment_indents": true,
			},
			input: `a:
    bb: c  # x`,
			expect: `a:
  bb: c    # x`,
		},
		{
			// Overflow fallback uses the pad_line_comments floor (3), not 1.
			name: "preserve comment indents fallback honors pad_line_comments",
			config: map[string]any{
				"indent":                   4,
				"pad_line_comments":        3,
				"preserve_comment_indents": true,
			},
			input: `a:
  b: c # x`,
			expect: `a:
    b: c   # x`,
		},
		{
			name: "trim trailing whitespace",
			config: map[string]any{
				"trim_trailing_whitespace": true,
			},
			input: `a: 1
b: 2    `,
			expect: `a: 1
b: 2`,
		},
		{
			name: "eof newline",
			config: map[string]any{
				"eof_newline": true,
			},
			input: `a: 1
b: 2`,
			expect: `a: 1
b: 2
`,
		},
		{
			name: "strip directives",
			config: map[string]any{
				"strip_directives": true,
			},
			input: "%YAML:1.0\na: 1",
		},
		{
			name: "strip directives keeps the document start marker after them",
			config: map[string]any{
				"strip_directives": true,
			},
			input:  "%YAML 1.2\n---\na: 1\n",
			expect: "%YAML 1.2\n---\na: 1\n",
		},
		{
			name: "strip directives keeps the end of the document before them",
			config: map[string]any{
				"strip_directives": true,
			},
			input:  "a: 1\n...\n%YAML 1.2\n---\nb: 2\n",
			expect: "a: 1\n...\n%YAML 1.2\n---\nb: 2\n",
		},
		{
			name: "strip directives adds no marker the input did not have",
			config: map[string]any{
				"strip_directives": true,
			},
			input:  "%YAML:1.0\na: 1\n%TAG:hi\nb: 2\n",
			expect: "%YAML:1.0\na: 1\n%TAG:hi\nb: 2\n",
		},
		{
			name: "strip directives keeps consecutive directives",
			config: map[string]any{
				"strip_directives":       true,
				"include_document_start": true,
			},
			input:  "%YAML 1.2\n%FOO bar\n---\na: 1\n",
			expect: "%YAML 1.2\n%FOO bar\n---\na: 1\n",
		},
		{
			name: "strip directives keeps consecutive trailing directives",
			config: map[string]any{
				"strip_directives": true,
			},
			input:  "a: 1\n%FOO bar\n%BAR baz\n",
			expect: "a: 1\n%FOO bar\n%BAR baz\n",
		},
		{
			name: "strip directives after retained line breaks",
			config: map[string]any{
				"strip_directives":       true,
				"retain_line_breaks":     true,
				"include_document_start": true,
			},
			input:  "%YAML 1.2\n---\na: 1\n\nb: 2\n",
			expect: "%YAML 1.2\n---\na: 1\n\nb: 2\n",
		},
		{
			name: "strip directives after trimmed whitespace",
			config: map[string]any{
				"strip_directives":         true,
				"trim_trailing_whitespace": true,
				"include_document_start":   true,
			},
			input:  "%YAML 1.2\n---\na: 1   \n",
			expect: "%YAML 1.2\n---\na: 1\n",
		},
		{
			name: "strip directives after the end-of-file newline",
			config: map[string]any{
				"strip_directives":       true,
				"eof_newline":            true,
				"include_document_start": true,
			},
			input:  "%YAML 1.2\n---\na: 1",
			expect: "%YAML 1.2\n---\na: 1\n",
		},
		{
			name: "array indent",
			config: map[string]any{
				"array_indent": 2,
			},
			input: `a:
    - 1
    - 2`,
			expect: `a:
  - 1
  - 2`,
		},
		{
			name: "indent root array",
			config: map[string]any{
				"indent_root_array": true,
				"array_indent":      2,
			},
			input: `  - 1
  - 2`,
			expect: `  - 1
  - 2`,
		},
		{
			name: "force block sequence style",
			config: map[string]any{
				"force_array_style": "block",
			},
			input: `a:
  - 1
  - 2
b: [1, 2]`,
			expect: `a:
  - 1
  - 2
b:
  - 1
  - 2`,
		},
		{
			name: "force flow sequence style",
			config: map[string]any{
				"force_array_style": "flow",
			},
			input: `a:
  - 1
  - 2
b: [1, 2]`,
			expect: `a: [1, 2]
b: [1, 2]`,
		},
		{
			name: "invalid sequence style",
			config: map[string]any{
				"force_array_style": "invalid",
			},
			badConfigErr: features.ErrUnrecognizedSequenceStyle,
		},
		{
			name: "force single quote style",
			config: map[string]any{
				"force_quote_style": "single",
			},
			input:  `a: ['hi', "hello"]`,
			expect: `a: ['hi', 'hello']`,
		},
		{
			name: "force double quote style",
			config: map[string]any{
				"force_quote_style": "double",
			},
			input:  `a: ['hi', "hello"]`,
			expect: `a: ["hi", "hello"]`,
		},
		{
			name: "invalid quote style",
			config: map[string]any{
				"force_quote_style": "blah",
			},
			badConfigErr: features.ErrUnrecognizedQuoteStyle,
		},
		{
			name: "alias key correction",
			input: `alias: &a something
map:
  *a: 1`,
			expect: `alias: &a something
map:
  *a : 1`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := factory.NewFormatter(tc.config)
			if tc.badConfigErr != nil {
				assert.Assert(t, errors.Is(err, tc.badConfigErr), "expected %v, got %v", tc.badConfigErr, err)
				return
			}

			assert.NilErr(t, err)

			result, err := f.Format([]byte(tc.input))
			if tc.formatErr {
				assert.NotNilErr(t, err)
				return
			}

			assert.NilErr(t, err)

			expected := tc.expect
			actual := string(result)

			if tc.expect == "" {
				expected = tc.input
			}

			if !tc.skipLineEndNormalization {
				// Having to always include the newline in the expected
				// data in the test table is cringe unless that's something
				// I actually want to test
				expected = stripTrailingNewline(expected)
				actual = stripTrailingNewline(actual)
			}

			assert.EqualMsg(t, expected, actual, outputMismatch)
		})
	}
}

// TestPreserveCommentIndentsIdempotent guards the property lint/check mode
// relies on: formatting an already-formatted file must be a no-op. After the
// first pass each comment's '#' sits at a column the parser reads back as its
// new source column, so a second pass must produce identical bytes.
func TestPreserveCommentIndentsIdempotent(t *testing.T) {
	testCases := []struct {
		name   string
		config map[string]any
		input  string
	}{
		{
			name:   "aligned columns",
			config: map[string]any{"preserve_comment_indents": true},
			input: `steps:
  - a    # first
  - bb   # second`,
		},
		{
			name:   "content grew, gap shrank",
			config: map[string]any{"indent": 4, "preserve_comment_indents": true},
			input: `a:
  bb: c    # x`,
		},
		{
			name:   "overflow fallback",
			config: map[string]any{"indent": 4, "preserve_comment_indents": true},
			input: `a:
  b: c # x`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := factory.NewFormatter(tc.config)
			assert.NilErr(t, err)

			once, err := f.Format([]byte(tc.input))
			assert.NilErr(t, err)
			twice, err := f.Format(once)
			assert.NilErr(t, err)

			assert.EqualMsg(
				t,
				string(once),
				string(twice),
				"second format pass changed the output.\nfirst:  %q\nsecond: %q",
			)
		})
	}
}

func TestRetainLineBreaks(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		expect string
		single bool
	}{
		{
			name: "basic",
			input: `a:  1

b: 2`,
			expect: `a: 1

b: 2
`,
		},
		{
			name: "multi-doc",
			input: `a:  1

# tail comment
---
b: 2`,
			expect: `a: 1

# tail comment
---
b: 2
`,
		},
		{
			name: "literal string",
			input: `a:  1

shell: |
  #!/usr/bin/env bash

  # hello, world
    # bye
  echo "hello, world"
`,
			expect: `a: 1

shell: |
  #!/usr/bin/env bash

  # hello, world
    # bye
  echo "hello, world"
`,
		},
		{
			name: "multi level nested literal string",
			input: `a:  1
x:
  y:
    shell: |
      #!/usr/bin/env bash

        # bye
      echo "hello, world"`,
			expect: `a: 1
x:
  y:
    shell: |-
      #!/usr/bin/env bash

        # bye
      echo "hello, world"
`,
		},
		{
			name:   "retain single line break",
			single: true,
			input: `a: 1

b: 2

c: 3
`,
			expect: `a: 1

b: 2

c: 3
`,
		},
		// A blank line inside a block scalar is its content, not a line
		// break to retain: a placeholder there folds into a folded scalar's
		// value, and a literal scalar's blank lines are preserved by the
		// encoder on their own.
		{
			name: "folded scalar with a blank line",
			input: `a: >
  one

  two
b: 1
`,
			expect: `a: >
  one

  two
b: 1
`,
		},
		{
			name:   "folded scalar with blank lines, single",
			single: true,
			input: `a: >-
  one


  two

b: 1
`,
			expect: `a: >-
  one


  two

b: 1
`,
		},
		{
			name: "sequence entry literal scalar",
			input: `- |
  one

  two
- key: >
    three

    four
  next: 1

- 2
`,
			expect: `- |
  one

  two
- key: >
    three

    four
  next: 1

- 2
`,
		},
		{
			name: "literal scalar followed by a blank line",
			input: `a: |
  one

b: 1
`,
			expect: `a: |
  one

b: 1
`,
		},
		{
			name: "keep chomping keeps trailing blank lines",
			input: `a: |+
  one


b: 1
`,
			expect: `a: |+
  one


b: 1
`,
		},
		// A blank line inside a multi-line quoted or plain scalar is the
		// value's: a placeholder comment folds into a quoted scalar and
		// ends a plain one. The encoder writes the value its own way, on
		// one line or as a literal block; the value is what must hold.
		{
			name: "double-quoted scalar with a blank line",
			input: `key: "foo

  bar"
next: 1
`,
			expect: `key: "foo\nbar"
next: 1
`,
		},
		{
			name:   "single-quoted scalar with blank lines, single",
			single: true,
			input: `key: 'it''s

  here'

next: 1
`,
			expect: `key: 'it''s

  here'

next: 1
`,
		},
		{
			name: "plain scalar with a blank line",
			input: `key: foo

  bar
next: 1
`,
			expect: `key: |-
  foo
  bar
next: 1
`,
		},
		{
			name: "plain scalar in a sequence, then a break",
			input: `- one
  two

  three

- four
`,
			expect: `- |-
  one two
  three

- four
`,
		},
		{
			name: "a value then a nested mapping after a break",
			input: `a: 1

b:

  c: 2
`,
			expect: `a: 1

b:

  c: 2
`,
		},
		{
			name: "a dash alone has no value",
			input: `-
  a: 1

  b: 2
`,
			expect: `- a: 1

  b: 2
`,
		},
		{
			name: "a plain scalar on the line after its key",
			input: `key:
  value continues

  here
next: 1
`,
			expect: `key: |-
  value continues
  here
next: 1
`,
		},
		{
			name: "a list of plain items under a key, with a break",
			input: `tags:
  - alpha

  - beta
`,
			expect: `tags:
  - alpha

  - beta
`,
		},
		{
			name: "a continued item then a break then the next",
			input: `key:
  - one
    two

  - three
`,
			expect: `key:
  - one two

  - three
`,
		},
		{
			name: "an indentless list under a key, with a break",
			input: `key:
- alpha

- beta
`,
			expect: `key:
  - alpha

  - beta
`,
		},
		{
			name: "a comment between a key and its plain value",
			input: `key:
  # c
  value

  more
next: 1
`,
			expect: `key: |-
  value
  more
# c
next: 1
`,
		},
		{
			name:  "a literal scalar at the end of an input without a final newline",
			input: "a: |\n  x",
			expect: `a: |-
  x
`,
		},
		{
			name:  "a line of spaces ends a literal scalar",
			input: "a: |\n  x\n   \nb: 1\n",
			expect: `a: "x\n \n"
b: 1
`,
		},
		{
			name:  "lines of spaces around an empty line end a literal scalar",
			input: "a: |\n  x\n   \n\n   \nb: 1\n",
			expect: `a: "x\n \n\n \n"
b: 1
`,
		},
		{
			name:  "an empty line then a line of spaces end a literal scalar",
			input: "a: |\n  x\n\n   \nb: 1\n",
			expect: `a: "x\n\n \n"
b: 1
`,
		},
		{
			name:  "a line of spaces ends a literal scalar at the end",
			input: "a: |\n  x\n   ",
			expect: `a: "x\n "
`,
		},
		{
			name: "indentation indicator",
			input: `a: |2
    one

  two
b: 1
`,
			expect: `a: |2
    one

  two
b: 1
`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := factory.NewFormatter(map[string]any{
				"retain_line_breaks":        true,
				"retain_line_breaks_single": tc.single,
			})
			assert.NilErr(t, err)
			got, err := f.Format([]byte(tc.input))
			assert.NilErr(t, err)
			assert.EqualMsg(t, tc.expect, string(got), outputMismatch)

			var before, after any

			assert.NilErr(t, yaml.Unmarshal([]byte(tc.input), &before))
			assert.NilErr(t, yaml.Unmarshal(got, &after))
			assert.Assert(
				t,
				reflect.DeepEqual(before, after),
				"formatting changed the document's value:\nbefore: %#v\nafter:  %#v",
				before,
				after,
			)
		})
	}
}

// TestRetainLineBreaksLongLine is a regression test for a line longer than
// bufio.Scanner's default 64KiB token limit. The retain-line-breaks feature
// scans the document line by line; before the fix this failed with
// "bufio.Scanner: token too long" on real YAML carrying a large single-line
// value (e.g. SwiftPM .build manifests). See internal/linescan.
func TestRetainLineBreaksLongLine(t *testing.T) {
	longValue := strings.Repeat("x", 200*1024) // 200KiB, well past the 64KiB limit
	input := "a: 1\n\nbig: " + longValue + "\nb: 2\n"

	for _, single := range []bool{false, true} {
		t.Run(fmt.Sprintf("single=%t", single), func(t *testing.T) {
			f, err := factory.NewFormatter(map[string]any{
				"retain_line_breaks":        true,
				"retain_line_breaks_single": single,
			})
			assert.NilErr(t, err)
			got, err := f.Format([]byte(input))
			assert.NilErr(t, err)
			assert.Assert(
				t,
				strings.Contains(string(got), longValue),
				"the long single-line value must survive formatting, got:\n%s",
				got,
			)
		})
	}
}

func stripTrailingNewline(s string) string {
	if trimmed, ok := strings.CutSuffix(s, "\r\n"); ok {
		return trimmed
	}

	return strings.TrimSuffix(s, "\n")
}

// TestFoldedScalarMoreIndentedLine: a folded scalar whose next line is more
// indented keeps its break on that indentation alone; the emitter must not
// add an empty line before it, which would add a newline to the value on
// every pass.
func TestFoldedScalarMoreIndentedLine(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"run: >-\n  echo \"a ${{\n    b }}\" >> out\n",
		"a: >\n  one\n    two\n  three\nb: 1\n",
		"a: >\n  one\n\n    two\n\n  three\nb: 1\n",
	}

	for _, config := range []map[string]any{{}, {"retain_line_breaks_single": true}} {
		f, err := factory.NewFormatter(config)
		assert.NilErr(t, err)

		for _, input := range inputs {
			got, err := f.Format([]byte(input))
			assert.NilErr(t, err)
			assert.EqualMsg(t, input, string(got), outputMismatch)

			var before, after any

			assert.NilErr(t, yaml.Unmarshal([]byte(input), &before))
			assert.NilErr(t, yaml.Unmarshal(got, &after))
			assert.Assert(
				t,
				reflect.DeepEqual(before, after),
				"formatting changed the value:\nbefore: %#v\nafter:  %#v",
				before,
				after,
			)
		}
	}
}

// TestBlockScalarLeadingBreak: a block scalar whose value opens with a
// newline keeps it as an empty first line, under an indentation indicator,
// in text and value, on every pass.
func TestBlockScalarLeadingBreak(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a: |\n\n  x\nb: 1\n":      "a: |2\n\n  x\nb: 1\n",
		"a: |2\n\n  x\nb: 1\n":     "a: |2\n\n  x\nb: 1\n",
		"a: >\n\n  x\nb: 1\n":      "a: >2\n\n  x\nb: 1\n",
		"a: |\n  x\n\n  y\nb: 1\n": "a: |\n  x\n\n  y\nb: 1\n",
		"a: |\n  x\nb: 1\n":        "a: |\n  x\nb: 1\n",
		// a comment on the indicator line ends it already
		"a: | # note\n  x\nb: 1\n":    "a: | # note\n  x\nb: 1\n",
		"a: >- # c\n  x\n  y\nb: 1\n": "a: >- # c\n  x y\nb: 1\n",
		"a: |+ # c\n  x\n\nb: 1\n":    "a: |+ # c\n  x\n\nb: 1\n",
		"a: | # note\n\n  x\nb: 1\n":  "a: |2 # note\n\n  x\nb: 1\n",
	}

	for _, config := range []map[string]any{{}, {"retain_line_breaks_single": true}} {
		f, err := factory.NewFormatter(config)
		if err != nil {
			t.Fatal(err)
		}

		for input, want := range cases {
			got, err := f.Format([]byte(input))
			assert.NilErr(t, err)
			assert.EqualMsg(t, want, string(got), outputMismatch)

			var before, after any

			assert.NilErr(t, yaml.Unmarshal([]byte(input), &before))
			assert.NilErr(t, yaml.Unmarshal(got, &after))
			assert.Assert(
				t,
				reflect.DeepEqual(before, after),
				"formatting changed the value:\nbefore: %#v\nafter:  %#v",
				before,
				after,
			)

			again, err := f.Format(got)
			assert.NilErr(t, err)
			assert.EqualMsg(t, string(got), string(again), "second pass differs.\nfirst:  %q\nsecond: %q")
		}
	}
}

// TestEmptyDocuments: a document with no content keeps its marker and its
// comments, so a stream keeps its document count through the formatter, on
// every pass, with and without retained line breaks and with explicit
// document starts.
func TestEmptyDocuments(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"---\n---\n":                  "---\n---\n",
		"---\n---\na: 1\n":            "---\n---\na: 1\n",
		"a: 1\n---\n":                 "a: 1\n---\n",
		"---\n":                       "---\n",
		"a: 1\n---\nb: 2\n":           "a: 1\n---\nb: 2\n",
		"---\na: 1\n---\n---\nb: 2\n": "a: 1\n---\n---\nb: 2\n",
		// an empty document's comments are kept, a comment on the marker's
		// own line coming back on a line of its own
		"---\n# c\n---\na: 1\n":   "---\n# c\n---\na: 1\n",
		"--- # c\n---\n":          "---\n# c\n---\n",
		"# top\n---\n---\na: 1\n": "# top\n---\n---\na: 1\n",
	}

	// With explicit document starts asked for, every document the encoder
	// writes opens with a marker of its own.
	explicit := map[string]string{
		"---\n---\n":                  "---\n---\n",
		"---\n---\na: 1\n":            "---\n---\na: 1\n",
		"a: 1\n---\n":                 "---\na: 1\n---\n",
		"---\n":                       "---\n",
		"a: 1\n---\nb: 2\n":           "---\na: 1\n---\nb: 2\n",
		"---\na: 1\n---\n---\nb: 2\n": "---\na: 1\n---\n---\nb: 2\n",
		"---\n# c\n---\na: 1\n":       "---\n# c\n---\na: 1\n",
	}

	configs := []struct {
		config map[string]any
		cases  map[string]string
	}{
		{map[string]any{}, cases},
		{map[string]any{"retain_line_breaks_single": true}, cases},
		{map[string]any{"include_document_start": true}, explicit},
	}

	for _, c := range configs {
		f, err := factory.NewFormatter(c.config)
		if err != nil {
			t.Fatal(err)
		}

		for input, want := range c.cases {
			got, err := f.Format([]byte(input))
			assert.NilErr(t, err)
			assert.EqualMsg(t, want, string(got), outputMismatch)

			if before, after := documentCount(t, []byte(input)), documentCount(t, got); before != after {
				t.Errorf("%q: %d documents in, %d out", input, before, after)
			}

			again, err := f.Format(got)
			assert.NilErr(t, err)
			assert.EqualMsg(t, string(got), string(again), "second pass differs.\nfirst:  %q\nsecond: %q")
		}
	}
}

// documentCount parses every document of a stream.
func documentCount(t *testing.T, stream []byte) int {
	t.Helper()

	dec := yaml.NewDecoder(bytes.NewReader(stream))
	n := 0

	for {
		var doc yaml.Node

		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return n
		}

		if err != nil {
			t.Fatal(err)
		}

		n++
	}
}
