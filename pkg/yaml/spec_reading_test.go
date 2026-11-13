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
