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
		// The same with nothing after the mapping, and after the key's own
		// foot: the pending head joins that foot, where written apart it
		// read back as the document's.
		{"inline value's head after the key's foot in a nested mapping", "- 0: \n#\n #\n  >\n", false, "- 0: \"\"\n  #\n  #\n"},
		// Of a pending head of several blocks only the first joins the key's
		// foot; the block after an empty line is the document's.
		{"inline value's two head blocks in a nested mapping", "- 0: \n\n#\n\n#\n  >\n", false, "- 0: \"\"\n  #\n\n#\n"},
		// A comment on a bare dash with no entry after it in its sequence
		// heads that null entry, as a null entry's line comment is written;
		// queued for whatever followed, it ended on the enclosing key as a
		// foot, written at that key's indent, and read back elsewhere.
		{"comment on a bare dash ending a nested sequence", "- 0:\n  - #00\n-\n", false, "- 0:\n    #00\n    -\n-\n"},
		{"comment on a bare dash ending a nested sequence under a key", "k:\n  - a\n  - # c\n", false, "k:\n    - a\n    # c\n    -\n"},
		{"comment on a bare dash ending a root sequence", "- a\n- # c\n", false, "- a\n# c\n-\n"},
		// A comment after an item whose mapping's last value is a nested
		// block goes down to that block's last leaf, written at its indent,
		// as the next pass reads a comment adjacent to the leaf.
		{"comment at the key's column after an indentless null item", "- 0:\n  -\n #\n-\n", false, "- 0:\n    -\n  #\n-\n"},
		{"comment below the key after an indentless item", "- 0:\n  - 0\n#\n\n-\n", false, "- 0:\n    - 0\n    #\n-\n"},
		// A comment at a key's column after its nested sequence, before a
		// sibling key, is the key's foot, set apart from the block.
		{"comment at the key's column after a nested item", "k:\n  - a\n# c\nj: 1\n", false, "k:\n    - a\n\n# c\nj: 1\n"},
		// A block after an empty line is no foot of the data before it,
		// even dedented: it heads what follows, set apart from the block.
		// Glued to the data it was the nested key's or item's foot.
		{"block after an empty line, dedented, before a sibling key", "a:\n  b: 1\n\n  # c\nd: 2\n", false, "a:\n    b: 1\n\n# c\nd: 2\n"},
		{"block after an empty line, dedented, before a sibling key of a sequence", "k:\n  - a\n\n  # c\nj: 1\n", false, "k:\n    - a\n\n# c\nj: 1\n"},
		{"block after an empty line, dedented, before a sibling item", "- 0:\n    - 1\n\n  # c\n- 2\n", false, "- 0:\n    - 1\n\n# c\n- 2\n"},
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
		// foot on the second. The comment before the marker is the
		// document's, written with the empty line that keeps it so.
		{"document head comment, marker, null item", "#0\n---\n-", false, "#0\n\n-\n"},
		{"head comment before a null item", "#0\n-", false, "#0\n-\n"},
		{"comment between two null items", "-\n#1\n-\n", false, "-\n#1\n-\n"},
		{"comment between two null items in a nested sequence", "k:\n  -\n  #1\n  -\n", false, "k:\n    -\n    #1\n    -\n"},
		// A comment on a bare dash's line heads what follows, with the
		// block after an empty line; written as a foot and a head, the two
		// read back as such. A foot after an item goes to that item.
		{"comment on a dash, empty line, comment, null item", "- #\n\n#\n-\n", false, "-\n#\n\n#\n-\n"},
		{"foot after an item then another item", "- 1\n# f\n\n- 2\n", false, "- 1\n# f\n\n- 2\n"},
		{"foot after the last item", "- 1\n- 2\n# f\n", false, "- 1\n- 2\n# f\n"},
		// The next entry's own foot unfolds with the prior entry's at the
		// peek past it; each goes to its entry.
		{"foot after each null item", "-\n#\n\n#\n-\n#\n", false, "-\n#\n\n#\n-\n#\n"},
		{"foot after each item", "- 1\n# a\n- 2\n# b\n", false, "- 1\n# a\n- 2\n# b\n"},
		// A root sequence's document foot is written adjacent, like the
		// last item's, which it reads back as.
		{"root sequence, empty line, comment", "- 1\n\n# f\n", false, "- 1\n# f\n"},
		// An entry's head comment after a nested block is set apart by an
		// empty line: adjacent, it read back as the block's dedented foot.
		{"head of an item after a nested mapping", "- a: 1\n\n# c\n- 2\n", false, "- a: 1\n\n# c\n- 2\n"},
		{"head of a key after a nested mapping", "a:\n    b: 1\n\n# c\nd: 2\n", false, "a:\n    b: 1\n\n# c\nd: 2\n"},
		{"two blocks heading an item after a nested mapping", "- 0:\n\n#\n\n#\n-\n", false, "- 0:\n\n#\n\n#\n-\n"},
		// A comment at the items' indent after an item that is a mapping
		// heads the next item, set apart from the mapping by an empty line.
		{"comment between two single-key items", "- 0:\n#\n- 0:\n", false, "- 0:\n\n#\n- 0:\n"},
		{"foot after a last item that is a mapping", "- a: 1\n#\n", false, "- a: 1\n  #\n"},
		// The same in an indentless sequence, whose null items took no
		// comments and lost them with the enclosing mapping's end.
		{"comment between two null items in an indentless sequence", "k:\n-\n#1\n-\n", false, "k:\n    -\n    #1\n    -\n"},
		{"head comment before a null item in an indentless sequence", "a:\n#0\n-\n", false, "a:\n    #0\n    -\n"},
		// An explicit null key takes the comments before it, as a key with
		// a node does; left pending they were lost the same way.
		{"head comment before an explicit null key", "#0\n? \n", false, "#0\nnull:\n"},
		// The comment on an explicit null key's own line is its line
		// comment; on the mapping's end it was written into what followed.
		{"explicit null key with a line comment", "? #c\n", false, "null: #c\n"},
		{"explicit null key with a line comment then a key", "? #c\na: 1\n", false, "null: #c\na: 1\n"},
		// The `?` line's comment heads a key whose node starts on the next
		// line; joined to the node's own line comment it came back as a foot.
		{"explicit key line comment then the key with its own", "? #a\n 0 #b\n", false, "#a\n0: #b\n"},
		{"explicit key line comment then the key", "? #a\n 0\n", false, "#a\n0:\n"},
		// A non-specific tag is not written back, so its line's comment has
		// no line: it heads the collection, mapping or sequence.
		{"tag line, explicit key line and key line comments", "! #a\n? #b\n 0 #c\n", false, "#a\n#b\n0: #c\n"},
		{"line comment on a sequence with a non-specific tag", "! #c\n- a\n", false, "#c\n- a\n"},
		{"line comment on a nested sequence with a non-specific tag", "- ! #c\n  - a\n", false, "#c\n- - a\n"},
		{"line comment on a sequence value with a non-specific tag", "k: ! #c\n  - a\n", false, "k:\n    #c\n    - a\n"},
		// A document's head comment is the one an empty line separates from
		// the content; written without the line it read back as the first
		// node's, which moved the comment and lost the line on every pass.
		{"two head comment blocks on a document", "#a\n\n#b\n\nk: 1\n", false, "#a\n\n#b\n\nk: 1\n"},
		// An empty node with properties, an anchor or a tag and no content
		// token, takes the heads before it and the line comment after them:
		// left pending, the head became the document's foot on the next
		// pass and the line comment the mapping's, which is not written.
		{"head comment before the marker of a tagged null", "#\n---\n!0000\n", false, "#\n\n!0000\n"},
		{"head comment before a tagged null", "#\n!0000\n", false, "#\n!0000\n"},
		{"line comment after a tagged empty value", "0: !0 #c\n", false, "0: !0 #c\n"},
		{"line comment after an anchored empty value", "0: &a #c\n", false, "0: &a #c\n"},
		// A null item written as nothing has no line for a line comment:
		// `- #` read back as a comment line; the comment heads the item.
		{"line comment on a null item with a non-specific tag", "- ! #c\n", false, "#c\n-\n"},
		{"line comment on a tagged null item", "- !t #c\n", false, "- !t #c\n"},
		{"line comment on an anchored null item", "- &a #c\n", false, "- &a #c\n"},
		// A block collection's properties, written on a line of their own,
		// carry the collection's line comment, which moved onto the first
		// key and then was lost with the tagged empty value it landed on.
		{"line comment on a tagged root mapping's tag line", "!!str #0000\n0: !0\n", false, "!!str #0000\n0: !0\n"},
		{"line comment on a nested tagged mapping's tag line", "a: !t # c\n  b: 1\n", false, "a: !t # c\n    b: 1\n"},
		{"line comment on an anchored sequence's anchor line", "&x # c\n- 1\n", false, "&x # c\n- 1\n"},
		{"line comment after a tagged flow mapping", "!!map {a: 1} # c\n", false, "!!map {a: 1} # c\n"},
		// Two line comments unfolded together, inside a flow mapping and
		// after it, are one line comment and a foot.
		{"comment inside a flow mapping and one after", "{?#\n}#\n", false, "{null: null} #\n#\n"},
		// A flow collection's own head comment has no line inside its
		// brackets, where it was written and then read back as nothing; it
		// is placed as the head of a value written on its key's line is.
		{"head comment on an empty flow mapping value", "0:\n#\n {}\n", false, "0: {}\n\n#\n"},
		{"head comment on a flow sequence value", "0:\n#\n [1]\n", false, "0: [1]\n\n#\n"},
		// Inside a flow mapping, the first comment block before the closing
		// brace is the last entry's foot and a block after an empty line
		// the mapping's, as written back; as one foot the second block
		// moved out on the next pass.
		{"two comment blocks inside a flow mapping", "{0\n\n#\n\n#\n}\n", false, "{0: null,\n    #\n}\n#\n"},
		// A key's line comment before a block value with a tag goes on the
		// tag's line, the value's; written first, the tag followed at no
		// indent and the output did not parse.
		{"key line comment then a tagged sequence value", "0: #\n !0\n-\n", false, "0: !0 #\n    -\n"},
		{"key line comment then a tagged mapping value", "0: #\n !0\n a: 1\n", false, "0: !0 #\n    a: 1\n"},
		// With a comment of its own on the tag line, the key's heads the key
		// and the tag keeps its own on the key's line; written first, the
		// tag followed at no indent and the output did not parse.
		{"key line comment then a tagged sequence with its own", "0: #\n !0 #\n-\n", false, "#\n0: !0 #\n    -\n"},
		// The tag line's comment is the mapping's; the one after the first
		// key's empty value, unfolded with it, is that key's.
		{"tag line comment and a value line comment", "!0 #\n0: #\n", false, "!0 #\n0: #\n"},
		{"tag alone on its line, value line comment", "!0\n0: #\n", false, "!0\n0: #\n"},
		// A comment on a scalar's properties line heads the scalar; joined
		// to the scalar's own line comment it came back as a foot.
		{"tag line comment then a scalar with its own", "! #a\n0 #b\n", false, "#a\n0 #b\n"},
		{"tag line comment then a scalar", "!t #a\n0\n", false, "#a\n!t 0\n"},
		// A comment on the line after a block collection's tag line heads
		// the first entry: the collection has no prior entry for a foot.
		{"tagged mapping, comment, first key", "!0\n#\nnull:\n", false, "!0\n#\nnull:\n"},
		{"tagged mapping, two blocks, explicit null key", "!0\n\n#\n\n#\n?\n", false, "!0\n#\n\n#\nnull:\n"},
		{"tagged sequence, comment, first item", "!!seq\n# c\n- a\n", false, "!!seq\n# c\n- a\n"},
		// A comment after an empty value at the stream's end is the key's
		// foot at its indent, as it is before a dedented sibling; as a head
		// with nothing to head it drifted to the document's end on the
		// second pass, where the explicit form had kept it on the first.
		{"nested explicit null key then a comment, no sibling", "a:\n  ?\n  #\n", false, "a:\n    null:\n    #\n"},
		{"nested explicit key then a comment, no sibling", "a:\n  ? b\n  #\n", false, "a:\n    b:\n    #\n"},
		{"nested empty value then a comment, no sibling", "a:\n  b:\n  #\n", false, "a:\n    b:\n    #\n"},
		// A comment after an explicit key with no value is that key's foot;
		// the decoder moved it to the prior key as a dedented foot, which
		// now reaches the prior entry as a tail of its own.
		{"explicit last key then a comment", "a: 1\n? 0\n#\n", false, "a: 1\n0:\n\n#\n"},
		{"two explicit null keys then a comment", "?\n? 0\n#\n", false, "null:\n0:\n\n#\n"},
		{"two explicit keys then a comment", "? 0\n? 1\n#\n", false, "0:\n1:\n\n#\n"},
		// A comment after an empty value, an empty line, then a key at the
		// same indent is the first key's foot, not the second's head with
		// the empty line in it.
		{"empty value, comment, empty line, sibling", "a:\n#\n\n0:\n", false, "a:\n#\n\n0:\n"},
		{"dedented foot before a sibling", "a:\n  b: 1\n# c\n\nd: 2\n", false, "a:\n    b: 1\n# c\n\nd: 2\n"},
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

// TestBlockScalarIndicatorUnderASequenceEntry: a block scalar whose content
// opens with whitespace carries an indentation indicator, and under a
// sequence entry the content is indented two past the entry whatever the
// configured indent, so the indicator is two there; the configured indent
// alone made a document that did not parse at indent 4.
func TestBlockScalarIndicatorUnderASequenceEntry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, in, want2, want4 string }{
		{"leading space", "- |2\n   a\n", "- |2\n   a\n", "- |2\n   a\n"},
		{"leading break", "- |\n\n  a\n", "- |2\n\n  a\n", "- |2\n\n  a\n"},
		{"leading break and space", "- |2\n\n   a\n", "- |2\n\n   a\n", "- |2\n\n   a\n"},
		{"nested", "k:\n  - |2\n     a\n", "k:\n  - |2\n     a\n", "k:\n    - |2\n       a\n"},
		{"two lines", "- |2\n   a\n  b\n", "- |2\n   a\n  b\n", "- |2\n   a\n  b\n"},
		{"as a value", "k: |2\n   a\n", "k: |2\n   a\n", "k: |4\n     a\n"},
		{"at the root", "|2\n   a\n", "|2\n   a\n", "|4\n     a\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for indent, want := range map[int]string{2: tc.want2, 4: tc.want4} {
				var n yaml.Node
				if err := yaml.NewDecoder(bytes.NewReader([]byte(tc.in))).Decode(&n); err != nil {
					t.Fatalf("decode %q: %v", tc.in, err)
				}

				var out bytes.Buffer

				enc := yaml.NewEncoder(&out)
				enc.SetIndent(indent)

				if err := enc.Encode(&n); err != nil {
					t.Fatalf("encode at %d: %v", indent, err)
				}

				if err := enc.Close(); err != nil {
					t.Fatal(err)
				}

				if out.String() != want {
					t.Errorf("indent %d: %q, want %q", indent, out.String(), want)
				}

				var again yaml.Node
				if err := yaml.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&again); err != nil {
					t.Fatalf("indent %d: the output %q does not parse: %v", indent, out.String(), err)
				}

				if again.Content[0].Kind != n.Content[0].Kind {
					t.Fatalf("indent %d: the output %q reads back as another kind", indent, out.String())
				}
			}
		})
	}
}

// TestHeadCommentAfterAnExplicitKeysValueIndicator: a key too long for a
// simple key is written `? key` then `:` on its own line, and the line counts
// as indented, so a head comment on the value's first entry was written
// after the `:`, where it read back as a line comment; it takes the next
// line, and reads back as the head it is.
func TestHeadCommentAfterAnExplicitKeysValueIndicator(t *testing.T) {
	t.Parallel()

	key := strings.Repeat("0", 129)
	in := key + ":\n\n#\n\n#\n-\n"
	want := "? " + key + "\n:\n    #\n\n    #\n    -\n"

	one := roundTrip(t, in, false)
	if one != want {
		t.Errorf("first pass %q, want %q", one, want)
	}

	if two := roundTrip(t, one, false); two != one {
		t.Errorf("second pass %q, first %q", two, one)
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
