package yaml_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

// The formatter's test campaign held Format to an independent parser: the
// output decodes to the same documents and formats to itself. These are the
// inputs it found, held here at the package, decode then encode.

func roundTrip(t *testing.T, in string, crlf bool) string {
	t.Helper()

	var n yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader([]byte(in))).Decode(&n); err != nil {
		t.Fatalf("decode %q: %v", in, err)
	}

	var out bytes.Buffer

	enc := yaml.NewEncoder(&out)
	if crlf {
		enc.SetLineBreakStyle(yaml.LineBreakStyleCRLF)
	}

	if err := enc.Encode(&n); err != nil {
		t.Fatalf("encode %q: %v", in, err)
	}

	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	return out.String()
}

// TestEncodeIdempotentWithComments: a document encoded once encodes to
// itself. A head comment left pending by a value written on its key's line
// is the mapping's foot comment, written with the separation it reads back
// with; a keep-chomped block scalar before the document's foot comment gets
// no separating empty line, which would be its content on the next pass;
// and under CRLF a comment between two keys is the next key's head, as
// under LF, not a foot of the prior one.
func TestEncodeIdempotentWithComments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		crlf bool
		want string
	}{
		{"comment after a trailing comment", "0: #\n#\n 0", false, "0: 0 #\n\n#\n"},
		{"keep-chomped scalar then a comment", "|+\n\n#", false, "|4+\n\n#\n"},
		{"keep-chomped value then a comment", "a: |+\n    x\n\n#", false, "a: |+\n    x\n\n#\n"},
		// An empty block scalar is written as "", and the comment after a
		// quoted scalar on the first line was the document's foot (separated)
		// where after a plain one it is the scalar's (not): the scanner now
		// takes content on the first line as content.
		{"empty literal then a comment", "|\n#", false, "\"\"\n#\n"},
		{"empty folded then a comment", ">\n#", false, "\"\"\n#\n"},
		{"empty literal item then a comment", "- |\n#", false, "- \"\"\n#\n"},
		{"empty literal value then a comment", "a: |\n#", false, "a: \"\"\n#\n"},
		{"quoted scalar on the first line then a comment", "\"x\"\n#", false, "\"x\"\n#\n"},
		{"plain scalar on the first line then a comment", "x\n#", false, "x\n#\n"},
		// An explicit null key's trailing comment is the key's foot in the
		// source and the document's once the key is written "null:", so at
		// the root mapping's end it is written as the document's foot. Only
		// there: in a nested mapping the comment stays where it is, before
		// the enclosing mapping's next key, as a key with a value keeps its
		// foot at its own indent.
		{"explicit null key then a comment", "?\n#", false, "null:\n\n#\n"},
		{"explicit null key after a key then a comment", "a:\n?\n#", false, "a:\nnull:\n\n#\n"},
		{"explicit null key after a sequence value then a comment", "0:\n- 0\n?\n#", false, "0:\n    - 0\nnull:\n\n#\n"},
		{"nested key with a value then its indented foot", "a:\n  b: 1\n  # c\n", false, "a:\n    b: 1\n    # c\n"},
		{"nested key with an empty value then a comment at the root", "a:\n  b:\n# c\n", false, "a:\n    b:\n# c\n"},
		{"nested empty value, its comment, then a sibling", "a:\n  b:\n  # c\nd: 1\n", false, "a:\n    b:\n    # c\nd: 1\n"},
		{"nested empty value after a value, its comment, then a sibling", "a:\n  b: 1\n  c:\n  # d\ne: 2\n", false, "a:\n    b: 1\n    c:\n    # d\ne: 2\n"},
		{"nested explicit null key, its comment, then a sibling", "a:\n  ?\n  # c\nd: 1\n", false, "a:\n    null:\n    # c\nd: 1\n"},
		{"inline value's head comment in a nested mapping, then a sibling", "a:\n  0: #\n  #\n   0\nd: 1\n", false, "a:\n    0: 0 #\n    #\nd: 1\n"},
		// A flow mapping's own foot comment is written after the closing
		// brace and takes no comma, where a comma put a comment inside the
		// mapping on a line of its own.
		{"flow mapping on the first line then a comment", "{a: 1}\n# c\n", false, "{a: 1}\n# c\n"},
		{"flow mapping with an empty value then a comment", "{a: }\n#", false, "{a: null}\n#\n"},
		{"flow mapping item then a comment", "- {a: }\n#", false, "- {a: null}\n#\n"},
		{"flow mapping after an empty line then a comment", "\n{a: 1}\n#", false, "{a: 1}\n#\n"},
		// A comment before a null sequence item heads the item, as it heads
		// an item with a node; left pending by the parser it was the
		// document's foot, so a document head comment before a marker and
		// a null item lost the marker on the first pass and moved to the
		// foot on the second.
		{"document head comment, marker, null item", "#0\n---\n-", false, "#0\n-\n"},
		{"head comment before a null item", "#0\n-", false, "#0\n-\n"},
		{"comment between two null items", "-\n#1\n-\n", false, "-\n#1\n-\n"},
		{"comment between two null items in a nested sequence", "k:\n  -\n  #1\n  -\n", false, "k:\n    -\n    #1\n    -\n"},
		{
			"CRLF comment between keys",
			"# head\r\na: 1 # trailing\r\n# between\r\nb:\r\n  # inner\r\n  c: 2\r\n",
			true,
			"# head\r\na: 1 # trailing\r\n# between\r\nb:\r\n    # inner\r\n    c: 2\r\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			one := roundTrip(t, tc.in, tc.crlf)
			if one != tc.want {
				t.Errorf("first pass %q, want %q", one, tc.want)
			}

			if two := roundTrip(t, one, tc.crlf); two != one {
				t.Errorf("second pass %q, first %q", two, one)
			}
		})
	}
}

// TestNullStaysNullAsKeyAndInFlow: a null written where the emitter quotes
// an empty scalar, as a mapping key or inside a flow collection, is written
// as "null" and reads back as null, where ” read back as the empty string;
// a null value in block style stays an empty value.
func TestNullStaysNullAsKeyAndInFlow(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"?", "null:\n"},
		{"000: {0}", "000: {0: null}\n"},
		{"[~, a]", "[~, a]\n"},
		{"a:", "a:\n"},
		{"a:\n  b:", "a:\n    b:\n"},
	} {
		got := roundTrip(t, tc.in, false)
		if got != tc.want {
			t.Errorf("%q encodes to %q, want %q", tc.in, got, tc.want)
		}

		var n yaml.Node
		if err := yaml.NewDecoder(strings.NewReader(got)).Decode(&n); err != nil {
			t.Fatal(err)
		}

		var tags []string

		var walk func(*yaml.Node)

		walk = func(n *yaml.Node) {
			if n.Kind == yaml.ScalarNode {
				tags = append(tags, n.Tag)
			}

			for _, c := range n.Content {
				walk(c)
			}
		}

		walk(&n)

		for _, tag := range tags {
			if tag == "!!str" && !strings.Contains(tc.in, "a") {
				t.Errorf("%q: the output %q reads back a string", tc.in, got)
			}
		}
	}
}

// TestCRLFCommentAttachment: the scanner reads a CRLF as one break, so a
// comment line followed directly by a key under CRLF is that key's head
// comment, as it is under LF.
func TestCRLFCommentAttachment(t *testing.T) {
	t.Parallel()

	for _, nl := range []string{"\n", "\r\n"} {
		in := strings.ReplaceAll("a: 1\n# between\nb: 2\n", "\n", nl)

		var n yaml.Node
		if err := yaml.NewDecoder(strings.NewReader(in)).Decode(&n); err != nil {
			t.Fatal(err)
		}

		m := n.Content[0]
		foot, head := m.Content[0].FootComment, m.Content[2].HeadComment

		if foot != "" || head != "# between" {
			t.Errorf("%q: a's foot %q, b's head %q; want the comment as b's head", in, foot, head)
		}
	}
}
