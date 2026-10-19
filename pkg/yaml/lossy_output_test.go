package yaml_test

import (
	"bytes"
	"testing"

	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

// TestLocalTagWithBangSuffixIsWrittenEscaped: the local tag whose suffix is
// "!" ("!%21" as written, "!<!!>" verbatim) is the tag "!!", which is not
// the "!!" shorthand; written as the secondary handle with no suffix it was
// "!!", which does not parse. It is written "!%21", the same tag, and the
// shorthands stay themselves.
func TestLocalTagWithBangSuffixIsWrittenEscaped(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"!%21", "!%21\n"},
		{"k: !%21 v", "k: !%21 v\n"},
		{"!<!!> x", "!%21 x\n"},
		{"!!str x", "!!str x\n"},
		{"!e%2Cf x", "!e,f x\n"},
	} {
		var n yaml.Node
		if err := yaml.NewDecoder(bytes.NewReader([]byte(tc.in))).Decode(&n); err != nil {
			t.Fatalf("decode %q: %v", tc.in, err)
		}

		var out bytes.Buffer

		enc := yaml.NewEncoder(&out)
		if err := enc.Encode(&n); err != nil {
			t.Fatalf("encode %q: %v", tc.in, err)
		}

		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}

		if out.String() != tc.want {
			t.Errorf("%q encodes to %q, want %q", tc.in, out.String(), tc.want)
		}

		var m yaml.Node
		if err := yaml.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&m); err != nil {
			t.Errorf("%q: the output %q does not parse: %v", tc.in, out.String(), err)
		} else if m.Content[0].Tag != n.Content[0].Tag {
			t.Errorf("%q: the output reads back with tag %q, want %q", tc.in, m.Content[0].Tag, n.Content[0].Tag)
		}
	}
}

// TestBlockScalarLedByATabKeepsItsIndentationIndicator: content opening with
// a tab needs the indentation indicator as content opening with a space
// does; written without it, the parser took the tab for indentation and
// refused the output ("found a tab character where an indentation space is
// expected"). A leading space kept its indicator already.
func TestBlockScalarLedByATabKeepsItsIndentationIndicator(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"|1\n \t", "|4-\n    \t\n"},
		{"|1\n \ta", "|4-\n    \ta\n"},
		{"k: |1\n \tx\n", "k: |4\n    \tx\n"},
		{">1\n \tx\n", ">4\n    \tx\n"},
		{"|2\n   a", "|4-\n     a\n"},
		{"|\n  a\n", "|\n    a\n"},
	} {
		var n yaml.Node
		if err := yaml.NewDecoder(bytes.NewReader([]byte(tc.in))).Decode(&n); err != nil {
			t.Fatalf("decode %q: %v", tc.in, err)
		}

		var out bytes.Buffer

		enc := yaml.NewEncoder(&out)
		if err := enc.Encode(&n); err != nil {
			t.Fatalf("encode %q: %v", tc.in, err)
		}

		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}

		if out.String() != tc.want {
			t.Errorf("%q encodes to %q, want %q", tc.in, out.String(), tc.want)
		}

		var m yaml.Node
		if err := yaml.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&m); err != nil {
			t.Errorf("%q: the output %q does not parse: %v", tc.in, out.String(), err)

			continue
		}

		if scalarText(&m) != scalarText(&n) {
			t.Errorf("%q: the output reads back as %q, want %q", tc.in, scalarText(&m), scalarText(&n))
		}
	}
}

// scalarText is the document's one scalar's value, or its one mapping's
// value's.
func scalarText(doc *yaml.Node) string {
	n := doc.Content[0]
	if n.Kind == yaml.MappingNode {
		n = n.Content[1]
	}

	return n.Value
}
