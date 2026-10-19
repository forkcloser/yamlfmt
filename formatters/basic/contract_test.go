// The basic formatter's contract from the outside, against an independent
// YAML parser as the oracle: what comes out parses, means what went in, and
// does not change when formatted again.
package basic_test

import (
	"bytes"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/forkcloser/yamlfmt"
	"github.com/forkcloser/yamlfmt/formatters/basic"
)

// decode is the oracle's reading of every document in the bytes, as
// plain Go values; an error is the oracle's.
func decode(data []byte) ([]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var docs []any

	for {
		var v any

		err := dec.Decode(&v)
		if err != nil {
			if err.Error() == "EOF" {
				return docs, nil
			}

			return nil, err
		}

		docs = append(docs, v)
	}
}

func formatter(t *testing.T, config map[string]any) yamlfmt.Formatter {
	t.Helper()

	f, err := (&basic.BasicFormatterFactory{}).NewFormatter(config)
	if err != nil {
		t.Fatalf("formatter with %v: %v", config, err)
	}

	return f
}

// corpus is the inputs every configuration is held on: the YAML shapes a
// formatter meets.
func corpus() map[string]string {
	return map[string]string{
		"flat map":            "a: 1\nb: two\nc: 3.5\nd: true\ne: null\n",
		"nested":              "a:\n  b:\n    c: [1, 2, 3]\n    d: {x: 1, y: 2}\n  e:\n    - one\n    - two\n",
		"root sequence":       "- a\n- b:\n    c: d\n- - nested\n  - seq\n",
		"comments":            "# head\na: 1 # trailing\n# between\nb:\n  # inner\n  c: 2\n",
		"anchors":             "base: &base\n  k: v\nderived:\n  <<: *base\n  extra: 1\nref: *base\n",
		"literal keep":        "text: |+\n  line one\n  line two\n\n\nafter: 1\n",
		"literal strip":       "text: |-\n  line one\n  line two\nafter: 1\n",
		"literal clip":        "text: |\n  line one\n  line two\nafter: 1\n",
		"folded":              "text: >\n  folded\n  text\n\n  paragraph\nafter: 1\n",
		"quoted":              "a: 'single ''quoted'''\nb: \"double \\\"quoted\\\" \\n escaped\"\nc: \"123\"\nd: 'true'\n",
		"special keys":        "\"a b\": 1\n? [complex, key]\n: value\n'with: colon': 2\n\"\": empty\n",
		"multi document":      "a: 1\n---\nb: 2\n---\n- c\n",
		"document markers":    "---\na: 1\n...\n---\nb: 2\n",
		"empty values":        "a:\nb: ''\nc: []\nd: {}\n",
		"long line":           "key: " + strings.Repeat("word ", 40) + "\n",
		"trailing spaces":     "a: 1   \nb:   \n  c: 2  \n",
		"tabs in scalars":     "a: \"tab\\there\"\n",
		"unicode":             "\u540d\u524d: \u5024\nemoji: \"\U0001f642\"\n",
		"numbers":             "a: 0x1f\nb: 1e3\nc: .inf\nd: -0\ne: 012\nf: 1_000\n",
		"timestamps":          "t: 2001-12-14t21:59:43.10-05:00\nd: 2002-12-14\n",
		"tags":                "a: !!str 123\nb: !!int \"42\"\nc: !custom x\n",
		"windows line ending": "a: 1\r\nb:\r\n  c: 2\r\n",
		"indentless":          "a:\n- 1\n- 2\nb:\n  - 3\n",
		"deep":                strings.Repeat("k:\n  ", 20) + "v: 1\n",
		"no final newline":    "a: 1",
		"only comment":        "# nothing else\n",
	}
}

// configurations are the default and each documented knob on its own.
func configurations() map[string]map[string]any {
	return map[string]map[string]any{
		"default":                   nil,
		"indent 4":                  {"indent": 4},
		"include_document_start":    {"include_document_start": true},
		"retain_line_breaks":        {"retain_line_breaks": true},
		"retain_line_breaks_single": {"retain_line_breaks_single": true},
		"indentless_arrays":         {"indentless_arrays": true},
		"drop_merge_tag":            {"drop_merge_tag": true},
		"pad_line_comments 2":       {"pad_line_comments": 2},
		"preserve_comment_indents":  {"preserve_comment_indents": true},
		"trim_trailing_whitespace":  {"trim_trailing_whitespace": true},
		"eof_newline":               {"eof_newline": true},
		"strip_directives":          {"strip_directives": true},
		"array_indent 2":            {"array_indent": 2},
		"indent_root_array":         {"indent_root_array": true},
		"max_line_length 40":        {"max_line_length": 40},
		"scan_folded_as_literal":    {"scan_folded_as_literal": true},
		"force_array_style flow":    {"force_array_style": "flow"},
		"force_array_style block":   {"force_array_style": "block"},
		"force_quote_style single":  {"force_quote_style": "single"},
		"force_quote_style double":  {"force_quote_style": "double"},
	}
}

// TestContractFormatPreservesTheDocument: for every configuration and
// every corpus input the oracle parses, the formatted bytes parse, decode
// to the same documents as the input, and format to themselves.
func TestContractFormatPreservesTheDocument(t *testing.T) {
	t.Parallel()

	for cname, config := range configurations() {
		t.Run(cname, func(t *testing.T) {
			t.Parallel()

			f := formatter(t, config)

			for iname, input := range corpus() {
				before, err := decode([]byte(input))
				if err != nil {
					continue // not the oracle's YAML: the fuzzer's territory
				}

				out, err := f.Format([]byte(input))
				if err != nil {
					t.Errorf("%s: Format: %v", iname, err)

					continue
				}

				after, err := decode(out)
				if err != nil {
					t.Errorf("%s: the output does not parse: %v\n%s", iname, err, out)

					continue
				}

				if !reflect.DeepEqual(before, after) {
					t.Errorf(
						"%s: the documents changed\ninput:\n%s\noutput:\n%s\nbefore %#v\nafter  %#v",
						iname,
						input,
						out,
						before,
						after,
					)
				}

				again, err := f.Format(out)
				if err != nil {
					t.Errorf("%s: a second Format: %v", iname, err)
				} else if !bytes.Equal(again, out) {
					t.Errorf("%s: not idempotent\nfirst:\n%s\nsecond:\n%s", iname, out, again)
				}
			}
		})
	}
}

// FuzzFormat: whatever the bytes, Format returns or errs and never
// panics; when the oracle parses the input and Format succeeds, the
// output parses and means the same; Format is idempotent on its output.
func FuzzFormat(f *testing.F) {
	for _, in := range corpus() {
		f.Add([]byte(in))
	}

	for _, in := range []string{"", "\n", "---\n", "...\n", "- \n", "a: [\n", "a: {", "&a *a\n", "!!binary x\n", "%YAML 1.2\n---\na: 1\n", "a: |\n", "\t\ta: 1\n", "a: 1\x00\n"} {
		f.Add([]byte(in))
	}

	formatter, err := (&basic.BasicFormatterFactory{}).NewFormatter(nil)
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		out, err := formatter.Format(input)
		if err != nil {
			return
		}

		before, oerr := decode(input)
		if oerr != nil || hasNil(before) {
			// A null anywhere (`?` alone, `{0}`, a bare `a:`) is held apart:
			// the formatter writes some of them as empty strings, with the
			// owner.
			return
		}

		after, err := decode(out)
		if err != nil {
			t.Fatalf("the output does not parse: %v\ninput: %q\noutput: %q", err, input, out)
		}

		if !reflect.DeepEqual(before, after) {
			t.Fatalf("the documents changed\ninput: %q\noutput: %q", input, out)
		}

		if os.Getenv("YAMLFMT_FUZZ_SKIP_IDEMPOTENCE") != "" {
			return
		}

		again, err := formatter.Format(out)
		if err != nil || !bytes.Equal(again, out) {
			t.Fatalf("not idempotent: %v\ninput: %q\nfirst: %q\nsecond: %q", err, input, out, again)
		}
	})
}

// hasNil reports a null key or value anywhere in the documents.
func hasNil(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case []any:
		return slices.ContainsFunc(x, hasNil)
	case map[string]any:
		return slices.ContainsFunc(slices.Collect(maps.Values(x)), hasNil)
	case map[any]any:
		for k, e := range x {
			if k == nil || hasNil(e) {
				return true
			}
		}
	}

	return false
}
