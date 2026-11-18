// What retaining line breaks keeps of a document that holds the text it
// marks empty lines with.

package basic_test

import (
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
