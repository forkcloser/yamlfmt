package yaml_test

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

// reading is an input, what its documents are, and how they are written.
type reading struct {
	name string
	in   string
	want []any
	out  string
}

// documents decodes every document of a stream to plain Go values.
func documents(t *testing.T, in string) []any {
	t.Helper()

	dec := yaml.NewDecoder(bytes.NewReader([]byte(in)))

	var docs []any

	for {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return docs
		}

		if err != nil {
			t.Fatalf("decode %q: %v", in, err)
		}

		docs = append(docs, v)
	}
}

// rewritten decodes every document of a stream to a node and encodes it.
func rewritten(t *testing.T, in string) string {
	t.Helper()

	dec := yaml.NewDecoder(bytes.NewReader([]byte(in)))

	var out bytes.Buffer

	enc := yaml.NewEncoder(&out)

	for {
		var n yaml.Node

		err := dec.Decode(&n)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("decode %q: %v", in, err)
		}

		if err := enc.Encode(&n); err != nil {
			t.Fatalf("encode %q: %v", in, err)
		}
	}

	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	return out.String()
}

// held holds each reading: the input is the documents it says, it is
// written as it says, and what is written is the same documents and is
// written the same again.
func held(t *testing.T, readings []reading) {
	t.Helper()

	for _, r := range readings {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()

			if got := documents(t, r.in); !reflect.DeepEqual(got, r.want) {
				t.Errorf("%q reads as %#v, want %#v", r.in, got, r.want)
			}

			out := rewritten(t, r.in)
			if out != r.out {
				t.Errorf("%q is written %q, want %q", r.in, out, r.out)
			}

			if got := documents(t, out); !reflect.DeepEqual(got, r.want) {
				t.Errorf("%q, written %q, reads back as %#v, want %#v", r.in, out, got, r.want)
			}

			if again := rewritten(t, out); again != out {
				t.Errorf("%q is written %q, then %q", r.in, out, again)
			}
		})
	}
}

// TestFlowEntriesWithAnEmptyKey: in a flow collection a pair's key may be
// empty, with or without its `?`, and a flow mapping's key may have its
// colon on a later line (YAML 1.2.2, 7.4.2). The parser wanted a key node
// before every value indicator, and a KEY token before every key with a
// value: each of these was refused.
func TestFlowEntriesWithAnEmptyKey(t *testing.T) {
	t.Parallel()

	held(t, []reading{
		{"a pair that is its colon alone", "[:]", []any{[]any{map[any]any{nil: nil}}}, "[{null: null}]\n"},
		{"a pair with a value alone", "[: x]", []any{[]any{map[any]any{nil: "x"}}}, "[{null: x}]\n"},
		{"such a pair after an item", "[a, : x]", []any{[]any{"a", map[any]any{nil: "x"}}}, "[a, {null: x}]\n"},
		{"a pair that is its question mark alone", "[? ]", []any{[]any{map[any]any{nil: nil}}}, "[{null: null}]\n"},
		{"an explicit empty key and a value", "[? : x]", []any{[]any{map[any]any{nil: "x"}}}, "[{null: x}]\n"},
		{"a mapping entry that is its colon alone", "{:}", []any{map[any]any{nil: nil}}, "{null: null}\n"},
		{"a mapping entry with a value alone", "{: 1}", []any{map[any]any{nil: 1}}, "{null: 1}\n"},
		{"such an entry before another", "{:,a: 1}", []any{map[any]any{nil: nil, "a": 1}}, "{null: null, a: 1}\n"},
		{"such an entry on its own line", "{\n:}", []any{map[any]any{nil: nil}}, "{null: null}\n"},
		{"a key and its colon on the next line", "{a\n: b}", []any{map[string]any{"a": "b"}}, "{a: b}\n"},
		{
			"a quoted key and its colon on the next line",
			"{\"a\"\n: b}",
			[]any{map[string]any{"a": "b"}},
			"{\"a\": b}\n",
		},
	})
}

// TestNonSpecificTagIsKept: `!` resolves a node by its kind alone, so a
// scalar under it is a string whatever its text looks like (YAML 1.2.2,
// 3.3.2). Read as no tag at all, `! 0` was the integer and was written
// back as one.
func TestNonSpecificTagIsKept(t *testing.T) {
	t.Parallel()

	held(t, []reading{
		{"a scalar", "! 0", []any{"0"}, "! 0\n"},
		{"a value", "v: ! 0", []any{map[string]any{"v": "0"}}, "v: ! 0\n"},
		{"an empty value", "v: !", []any{map[string]any{"v": ""}}, "v: !\n"},
		{"items", "- ! true\n- ! ~", []any{[]any{"true", "~"}}, "- ! true\n- ! ~\n"},
		{
			"a key that is not the merge key",
			"! <<: {a: 1}",
			[]any{map[string]any{"<<": map[string]any{"a": 1}}},
			"! <<: {a: 1}\n",
		},
		{"empty in a flow sequence", "[! , ! a]", []any{[]any{"", "a"}}, "[! '', ! a]\n"},
		{"a sequence", "! [1, 2]", []any{[]any{1, 2}}, "! [1, 2]\n"},
		{"a mapping", "!\nk: v\n", []any{map[string]any{"k": "v"}}, "!\nk: v\n"},
	})
}

// TestNonSpecificTagShortTag: the node says what the tag makes it.
func TestNonSpecificTagShortTag(t *testing.T) {
	t.Parallel()

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("! 0"), &doc); err != nil {
		t.Fatal(err)
	}

	if got := doc.Content[0].ShortTag(); got != "!!str" {
		t.Errorf("the short tag of `! 0` is %q, want !!str", got)
	}
}

// TestDocumentBlockScalarContentAtColumnZero: a document's own block scalar
// is inside no collection, so its content may start at column 0 (YAML
// 1.2.2, 9.1.3, example 9.3), up to the next document marker. Held to
// column 1, a `#` line there was a comment and the content was lost.
func TestDocumentBlockScalarContentAtColumnZero(t *testing.T) {
	t.Parallel()

	held(t, []reading{
		{"literal", "|\n#", []any{"#"}, "|-\n    #\n"},
		{"folded", ">\n#", []any{"#"}, ">-\n    #\n"},
		{"after an empty line", "|+\n\n#", []any{"\n#"}, "|4-\n\n    #\n"},
		{
			"the specification's bare document",
			"|\n%!PS-Adobe-2.0 # Not the first line\n",
			[]any{"%!PS-Adobe-2.0 # Not the first line\n"},
			"|\n    %!PS-Adobe-2.0 # Not the first line\n",
		},
		{"more indented lines", "|\n\n\nx\n y\n", []any{"\n\nx\n y\n"}, "|4\n\n\n    x\n     y\n"},
		{"a marker ends it", "--- |\nx\n--- |\ny\n", []any{"x\n", "y\n"}, "|\n    x\n---\n|\n    y\n"},
		{"a marker and a plain document", "|\na\n---\nb\n", []any{"a\n", "b"}, "|\n    a\n---\nb\n"},
		{"an end marker and a comment", "--- |\n# c\n...\n# after\n", []any{"# c\n"}, "|\n    # c\n# after\n"},
		{"three dashes that are no marker", "|\n---x\n", []any{"---x\n"}, "|\n    ---x\n"},
		{"a value's is deeper than its key", "a: |\n#x\n", []any{map[string]any{"a": ""}}, "a: \"\"\n#x\n"},
		{"an item's is deeper than its dash", "- |\n#x\n", []any{[]any{""}}, "- \"\"\n#x\n"},
	})
}

// TestFlowIndicatorsAreContentWhereTheyIndicateNothing: in a flow
// collection as in a block one, `?` is the key indicator only before a
// blank, a `:` ends a plain scalar also before a flow indicator, and a `:`
// where a node starts, before scalar content, starts a scalar (YAML 1.2.2,
// 7.3.3). Each read otherwise changed what was written: `[0:]` came back
// as the string "0:", `[?x]` as a pair with the key "x".
func TestFlowIndicatorsAreContentWhereTheyIndicateNothing(t *testing.T) {
	t.Parallel()

	held(t, []reading{
		{"a pair's key before the bracket", "[0:]", []any{[]any{map[any]any{0: nil}}}, "[{0: null}]\n"},
		{"a key before the brace", "{a:}", []any{map[string]any{"a": nil}}, "{a: null}\n"},
		{"a pair's key before the comma", "[a:, b]", []any{[]any{map[string]any{"a": nil}, "b"}}, "[{a: null}, b]\n"},
		{"a question mark starting a scalar", "[?x]", []any{[]any{"?x"}}, "['?x']\n"},
		{"a question mark starting a key", "{?foo: bar}", []any{map[string]any{"?foo": "bar"}}, "{'?foo': bar}\n"},
		{"a question mark inside a scalar", "[a?b]", []any{[]any{"a?b"}}, "['a?b']\n"},
		{"a colon starting a scalar", "[:x]", []any{[]any{":x"}}, "[':x']\n"},
		{"a colon starting a value", "{a: :x}", []any{map[string]any{"a": ":x"}}, "{a: ':x'}\n"},
		{"a colon starting a scalar after an item", "[a, :x]", []any{[]any{"a", ":x"}}, "[a, ':x']\n"},
		{"a colon starting an explicit key", "[? :x]", []any{[]any{map[string]any{":x": nil}}}, "[{':x': null}]\n"},
		{"a colon starting a mapping's key", "{:x}", []any{map[string]any{":x": nil}}, "{':x': null}\n"},
		{"a colon inside a scalar", "[a:b]", []any{[]any{"a:b"}}, "['a:b']\n"},
		{
			"a colon before an entry's end after an item",
			"[a, :]",
			[]any{[]any{"a", map[any]any{nil: nil}}},
			"[a, {null: null}]\n",
		},
		{"a key that ends before its colon on the next line", "{a\n:}", []any{map[string]any{"a": nil}}, "{a: null}\n"},
		{"an adjacent value", "{\"a\":b}", []any{map[string]any{"a": "b"}}, "{\"a\": b}\n"},
		{"an adjacent pair", "[\"a\":b]", []any{[]any{map[string]any{"a": "b"}}}, "[{\"a\": b}]\n"},
		{"an explicit key", "[? a: b]", []any{[]any{map[string]any{"a": "b"}}}, "[{a: b}]\n"},
	})
}

// TestAnchorNamesTakeEveryCharacterButBlanksAndFlowIndicators: an anchor's
// name ends at a blank or a flow indicator, not at the first character
// that is no letter, digit, `-` or `_` (YAML 1.2.2, 6.9.2). Ended early,
// `&an:chor value` was the anchor "an" on the scalar ":chor value". A
// colon stays out of a name it would end, where it is the value indicator.
func TestAnchorNamesTakeEveryCharacterButBlanksAndFlowIndicators(t *testing.T) {
	t.Parallel()

	// A run of colons longer than the scanner's buffer, which grows to
	// look past it.
	colons := "&a" + string(bytes.Repeat([]byte(":"), 1536)) + "b x"

	held(t, []reading{
		{"a run of colons past the buffer", colons, []any{"x"}, colons + "\n"},
		{"a colon inside", "k: &an:chor value\n", []any{map[string]any{"k": "value"}}, "k: &an:chor value\n"},
		{"a question mark inside", "&a?b x", []any{"x"}, "&a?b x\n"},
		{"an alias with the same name", "- &a:b x\n- *a:b\n", []any{[]any{"x", "x"}}, "- &a:b x\n- *a:b\n"},
		{"an alias as a key", "- &a x\n- *a: y\n", []any{[]any{"x", map[string]any{"x": "y"}}}, "- &a x\n- *a: y\n"},
		{"an alias in a flow sequence", "[&a x, *a]", []any{[]any{"x", "x"}}, "[&a x, *a]\n"},
		{
			"an alias before the brace",
			"{a: &b x, c: *b}",
			[]any{map[string]any{"a": "x", "c": "x"}},
			"{a: &b x, c: *b}\n",
		},
	})

	// A flow collection's start is no part of a name either, and nothing
	// else may follow a name directly.
	for _, in := range []string{"&a{b: c}", "&a[b]"} {
		var v any
		if err := yaml.Unmarshal([]byte(in), &v); err == nil {
			t.Errorf("%q is read: %#v", in, v)
		}
	}
}

// TestAnchorNameEndingInAColonIsNotWritten: written, the colon would be
// read back as the value indicator and the name as another.
func TestAnchorNameEndingInAColonIsNotWritten(t *testing.T) {
	t.Parallel()

	for _, anchor := range []string{"a:", ":", "a b", "a,b"} {
		node := yaml.Node{Kind: yaml.ScalarNode, Value: "x", Anchor: anchor}

		out, err := yaml.Marshal(&node)
		if err == nil {
			t.Errorf("the anchor %q is written: %q", anchor, out)
		}
	}

	node := yaml.Node{Kind: yaml.ScalarNode, Value: "x", Anchor: "a:b"}

	out, err := yaml.Marshal(&node)
	if err != nil || string(out) != "&a:b x\n" {
		t.Errorf("the anchor \"a:b\" is written %q, %v", out, err)
	}
}

// TestTagShorthandEndsAtAFlowIndicator: a shorthand tag holds no flow
// indicator (YAML 1.2.2, 5.6), so in a flow collection the indicator
// after a tag ends the tagged node, an empty one. Taken into the tag,
// `[!t,a]` did not parse, and `[!, ]` was written as two items. A tag that
// holds one is written with it escaped, or verbatim as it was.
func TestTagShorthandEndsAtAFlowIndicator(t *testing.T) {
	t.Parallel()

	held(t, []reading{
		{"before an item", "[!t,a]", []any{[]any{"", "a"}}, "[!t '', a]\n"},
		{"the non-specific tag", "[!, a]", []any{[]any{"", "a"}}, "[! '', a]\n"},
		{"before the bracket", "[!t]", []any{[]any{""}}, "[!t '']\n"},
		{"before the brace", "{a: !t}", []any{map[string]any{"a": ""}}, "{a: !t ''}\n"},
		{"empty values", "{a: !, b: ! 0}", []any{map[string]any{"a": "", "b": "0"}}, "{a: ! '', b: ! 0}\n"},
		{"an escaped comma", "!e%2Cf x", []any{"x"}, "!e%2Cf x\n"},
		{"an escaped comma in a flow sequence", "[!e%2Cf x]", []any{[]any{"x"}}, "[!e%2Cf x]\n"},
		{"a verbatim comma", "[!<a,b> x]", []any{[]any{"x"}}, "[!<a,b> x]\n"},
	})
}
