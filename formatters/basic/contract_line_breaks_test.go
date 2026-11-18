// What retaining line breaks keeps of a document that holds the text it
// marks empty lines with.

package basic_test

import (
	"reflect"
	"strings"
	"testing"
)

// TestContractRetainLineBreaksKeepsItsOwnMarkAsText: empty lines go through
// the formatter as comments of a fixed text, read back as empty lines after
// it. A document that holds that text itself, as a comment or in a scalar,
// keeps it: its lines are not read back as empty ones, and its empty lines
// are still retained.
func TestContractRetainLineBreaksKeepsItsOwnMarkAsText(t *testing.T) {
	t.Parallel()

	const mark = "#magic___^_^___line"

	documents := []string{
		"a: 1\n" + mark + " is the user's own comment\nb: 2\n",
		"a: 1\n\n" + mark + "\n\nb: 2\n",
		"a: |\n  " + mark + "\n  x\n\nb: 2\n",
		"a: 1 " + mark + "\n\nb: 2\n",
		// The text with more after it, the way a longer mark would be made.
		mark + "_\n\n" + mark + "__ x\n\n" + mark + "\na: 1\n",
	}

	for _, option := range []string{"retain_line_breaks", "retain_line_breaks_single"} {
		f := formatter(t, map[string]any{option: true})

		for _, document := range documents {
			out, err := f.Format([]byte(document))
			if err != nil {
				t.Fatalf("%s: %q: %v", option, document, err)
			}

			if string(out) != document {
				t.Errorf("%s: %q was written as %q", option, document, out)
			}
		}
	}
}

// TestContractRetainLineBreaksKeepsTheFinalLineBreak: a file that ends with
// a line break is read as one that does, whichever line ending it has and
// whichever is configured, and one that does not as one that does not: a
// block scalar that ends the file keeps its final line break, or its lack
// of one, and reads back as it read before.
func TestContractRetainLineBreaksKeepsTheFinalLineBreak(t *testing.T) {
	t.Parallel()

	documents := []string{
		"a: |\n  text\n",
		"a: |-\n  text\n",
		"a: |+\n  text\n\n",
		"a: >\n  folded\n",
		"- >-\n  folded\n",
		"a: |-\n  text",
		"a: 1\n\nb: |\n  text\n",
	}

	for _, option := range []string{"retain_line_breaks", "retain_line_breaks_single"} {
		for _, ending := range []string{"lf", "crlf"} {
			f := formatter(t, map[string]any{option: true, "line_ending": ending})

			for _, document := range documents {
				for _, input := range []string{document, strings.ReplaceAll(document, "\n", "\r\n")} {
					meant, err := decode([]byte(input))
					if err != nil {
						t.Fatalf("%q: %v", input, err)
					}

					out, err := f.Format([]byte(input))
					if err != nil {
						t.Fatalf("%s, %s: %q: %v", option, ending, input, err)
					}

					if read, err := decode(out); err != nil || !reflect.DeepEqual(read, meant) {
						t.Errorf("%s, %s: %q was written as %q, which reads back as %#v (%v), want %#v",
							option, ending, input, out, read, err, meant)
					}
				}
			}
		}
	}

	out, err := formatter(
		t,
		map[string]any{"retain_line_breaks": true, "line_ending": "crlf"},
	).Format([]byte("a: |\n  text\n"))
	if err != nil || string(out) != "a: |\r\n  text\r\n" {
		t.Errorf("a file with LF endings under crlf: %q (%v)", out, err)
	}
}
