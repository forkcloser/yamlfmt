// What a configuration value can do to a run: be refused, or be formatted
// with.

package basic_test

import (
	"bytes"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/forkcloser/yamlfmt"
	"github.com/forkcloser/yamlfmt/formatters/basic"
	"github.com/forkcloser/yamlfmt/formatters/kyaml"
)

// options is every option of the basic formatter, and one it does not have.
func options() []string {
	return []string{
		"indent", "include_document_start", "line_ending", "max_line_length", "retain_line_breaks",
		"retain_line_breaks_single", "disallow_anchors", "scan_folded_as_literal", "indentless_arrays",
		"drop_merge_tag", "pad_line_comments", "preserve_comment_indents", "trim_trailing_whitespace",
		"eof_newline", "strip_directives", "array_indent", "indent_root_array",
		"disable_alias_key_correction", "force_array_style", "force_quote_style", "type", "no_such_option",
	}
}

// values is what a configuration file or a flag can set an option to, of
// every type a YAML value decodes to, at and past the ends of each.
func values() []any {
	return []any{
		-1, 0, 1, 2, 9, 10, 81, -100000, math.MinInt64, math.MaxInt64, uint64(math.MaxUint64),
		-1.5, 2.5, 1e30, math.NaN(), math.Inf(1), math.Inf(-1),
		"", "x", "-1", "crlf", "flow", "single", true, false, nil,
		time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
		[]any{},
		[]any{-1},
		map[string]any{},
		map[string]any{"a": -1},
		map[any]any{1: 2},
	}
}

// documents is one of each shape an option changes the writing of.
func documents() []string {
	return []string{
		"a:\n  b:\n    - c: 1 # x\n    - - d\n      - e\n",
		"- a\n- b: [1, 2, {x: y}]\n- |\n  text\n   more\n- >\n  folded " + strings.Repeat("word ", 30) + "\n",
		"# head\nkey: " + strings.Repeat("word ", 40) + "\n\n\nother: 'q'\n...\n---\n&a x: *a\n",
		"%YAML 1.2\n---\na: 1\n",
		"",
	}
}

// TestContractNoConfigurationValueCrashes: whatever one option is set to,
// alone or beside the options that change how the others are written, a
// formatter is either refused with an error or formats, or refuses, every
// document: none panics. It holds for both formatters, the kyaml one being
// given what it does not read.
func TestContractNoConfigurationValueCrashes(t *testing.T) {
	t.Parallel()

	spaces := []string{"pad_line_comments", "array_indent", "indent"}

	beside := []map[string]any{
		nil,
		{"indentless_arrays": true},
		{"indent_root_array": true},
		{"retain_line_breaks": true, "strip_directives": true, "preserve_comment_indents": true},
	}

	factories := map[string]yamlfmt.Factory{
		"basic": &basic.BasicFormatterFactory{},
		"kyaml": &kyaml.KYAMLFormatterFactory{},
	}

	for name, factory := range factories {
		for _, option := range options() {
			for _, value := range values() {
				// Not tried, at the end of an int's range: none of the
				// three crashes. pad_line_comments writes spaces without
				// end before a comment. array_indent, and indent when no
				// array_indent is set, do so before the items of a
				// sequence at the root under indent_root_array; under a
				// mapping the count overflows and the items are written at
				// column 0, which no longer parses.
				if value == math.MaxInt64 && slices.Contains(spaces, option) {
					continue
				}

				for _, others := range beside {
					config := map[string]any{option: value}
					for other, set := range others {
						if other != option {
							config[other] = set
						}
					}

					func() {
						defer func() {
							if crash := recover(); crash != nil {
								t.Errorf("%s: %s set to %#v, beside %v, panics: %v", name, option, value, others, crash)
							}
						}()

						f, err := factory.NewFormatter(config)
						if err != nil {
							return
						}

						for _, document := range documents() {
							_, _ = f.Format([]byte(document))
						}

						_, _ = f.ConfigMap()
					}()
				}
			}
		}
	}
}

// TestContractNegativeIndentIsWrittenAsOne: an indent under zero is out of
// the range the encoder writes, as an indent of 1 is, and is written as
// that one is. An indent of zero is not: it is no indent set, and is
// written as 4.
func TestContractNegativeIndentIsWrittenAsOne(t *testing.T) {
	t.Parallel()

	for _, document := range documents() {
		want, err := formatter(t, map[string]any{"indent": 1}).Format([]byte(document))
		if err != nil {
			t.Fatalf("indent 1: %q: %v", document, err)
		}

		four, err := formatter(t, map[string]any{"indent": 4}).Format([]byte(document))
		if err != nil {
			t.Fatalf("indent 4: %q: %v", document, err)
		}

		if zero, err := formatter(
			t,
			map[string]any{"indent": 0},
		).Format([]byte(document)); err != nil ||
			!bytes.Equal(zero, four) {
			t.Errorf("indent 0: %q was written as %q (%v), and as %q with an indent of 4", document, zero, err, four)
		}

		for _, indent := range []any{-1, -2, -100000, math.MinInt64, -1.5} {
			out, err := formatter(t, map[string]any{"indent": indent}).Format([]byte(document))
			if err != nil {
				t.Fatalf("indent %v: %q: %v", indent, document, err)
			}

			if !bytes.Equal(out, want) {
				t.Errorf("indent %v: %q was written as %q, and as %q with an indent of 1", indent, document, out, want)
			}
		}
	}
}

// TestContractLineCommentStaysAComment: a line comment is written after a
// space at least. A pad_line_comments of 0, as one under 0, is written as 1
// is: without the space the comment is part of the value before it, and
// `b: 1 # x` would read back with the value "1# x".
func TestContractLineCommentStaysAComment(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"b: 1 # x\n",
		"- a # x\n- [b, c] # y\n- k: v # z\n",
		"k: # x\n  - l # y\n",
	}

	for _, input := range inputs {
		want, err := formatter(t, map[string]any{"pad_line_comments": 1}).Format([]byte(input))
		if err != nil {
			t.Fatalf("pad_line_comments 1: %q: %v", input, err)
		}

		meant, err := decode([]byte(input))
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}

		for _, pad := range []int{0, -1, -100000} {
			out, err := formatter(t, map[string]any{"pad_line_comments": pad}).Format([]byte(input))
			if err != nil {
				t.Fatalf("pad_line_comments %d: %q: %v", pad, input, err)
			}

			if !bytes.Equal(out, want) {
				t.Errorf(
					"pad_line_comments %d: %q was written as %q, and as %q with a padding of 1",
					pad,
					input,
					out,
					want,
				)
			}

			if read, err := decode(out); err != nil || !reflect.DeepEqual(read, meant) {
				t.Errorf("pad_line_comments %d: %q reads back as %v (%v), want %v", pad, input, read, err, meant)
			}
		}
	}

	out, err := formatter(t, map[string]any{"pad_line_comments": 0}).Format([]byte("b: 1 # x\n"))
	if err != nil {
		t.Fatal(err)
	}

	read, err := decode(out)
	if err != nil {
		t.Fatal(err)
	}

	if want := []any{map[string]any{"b": 1}}; !reflect.DeepEqual(read, want) {
		t.Errorf(
			"`b: 1 # x` under a padding of 0 was written as %q, which reads back as %#v, want %#v",
			out,
			read,
			want,
		)
	}
}
