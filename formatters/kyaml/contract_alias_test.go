// What the kyaml formatter does with aliases.
package kyaml_test

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/forkcloser/yamlfmt/formatters/kyaml"
)

// TestContractKYAMLRefusesAnAnchorThatHoldsItself: an alias inside the node
// its anchor names has no KYAML form, since the copy would hold a copy of
// itself without end. Such a document is refused with an error that names
// the alias and its line, and comes back as it went in. The encoder followed
// the alias until the stack ran out, and the run ended there.
//
//nolint:paralleltest // caps the stack of the whole process
func TestContractKYAMLRefusesAnAnchorThatHoldsItself(t *testing.T) {
	// The cap on what a formatter that follows the alias anyway can take
	// with it: a stack overflow at 32 MiB, not at the default gigabyte.
	defer debug.SetMaxStack(debug.SetMaxStack(32 << 20))

	cases := map[string]string{
		"a: &a [*a]\n":                   "line 1",
		"&a {b: *a}\n":                   "line 1",
		"&a\n- - b\n  - c: {d: [*a]}\n":  "line 3",
		"x: 1\n---\ny: &y\n  z:\n  - *y": "line 5",
		"&a [&b [*a]]\n":                 "line 1",
	}

	for input, line := range cases {
		out, err := (&kyaml.KYAMLFormatter{}).Format([]byte(input))
		if err == nil {
			t.Errorf("%q was written as %q", input, out)

			continue
		}

		if !strings.Contains(err.Error(), line) || !strings.Contains(err.Error(), "*") {
			t.Errorf("%q: the error names neither the alias nor %s: %v", input, line, err)
		}

		if string(out) != input {
			t.Errorf("%q: refused, and yet returned as %q", input, out)
		}
	}
}

// TestContractKYAMLWritesAnAliasAsACopy: every other alias is written, as a
// copy of what its anchor holds: an alias to an anchor that is complete, in
// its own document or an earlier one, or to a name its own value defines
// again for something else.
func TestContractKYAMLWritesAnAliasAsACopy(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ input, want string }{
		"alias": {
			"a: &a [1]\nb: *a\n",
			"---\n{\n  a: [\n    1,\n  ],\n  b: [\n      1,\n    ],\n}\n",
		},
		"alias to an earlier document": {
			"&a x\n---\n*a\n",
			"---\n\"x\"\n---\n\"x\"\n",
		},
		"name defined again inside": {
			"&a [&a x, *a]\n",
			"---\n[\n  \"x\",\n  \"x\",\n]\n",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := (&kyaml.KYAMLFormatter{}).Format([]byte(c.input))
			if err != nil {
				t.Fatalf("%q: %v", c.input, err)
			}

			if string(out) != c.want {
				t.Errorf("%q was written as %q, want %q", c.input, out, c.want)
			}
		})
	}
}
