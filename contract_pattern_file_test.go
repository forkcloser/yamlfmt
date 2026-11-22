// What a pattern file is read as, whatever its lines.

package yamlfmt_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"testing/iotest"

	"github.com/forkcloser/yamlfmt"
)

// TestContractPatternFileIsReadWhole: every line of a pattern file is a
// pattern, whatever the length of the lines before it; a file that cannot
// be read to its end is an error of the collection, not a shorter list of
// patterns.
func TestContractPatternFileIsReadWhole(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{"a.yaml": &fstest.MapFile{Data: []byte("a: 1\n")}}

	// 60 KiB is under the 64 KiB a line reader takes by default, 70 KiB
	// and 1 MiB are over.
	for _, long := range []int{60 << 10, 70 << 10, 1 << 20} {
		patterns := "# " + strings.Repeat("x", long) + "\n*.yaml\n"

		paths, err := yamlfmt.NewPatternFileCollectorFS(strings.NewReader(patterns), tree).CollectPaths()
		if err != nil {
			t.Fatalf("after a line of %d bytes: %v", long, err)
		}

		if !slices.Equal(paths, []string{"a.yaml"}) {
			t.Errorf("after a line of %d bytes the pattern collects %v, want a.yaml", long, paths)
		}
	}

	reader := io.MultiReader(strings.NewReader("*.yaml\n"), iotest.ErrReader(io.ErrClosedPipe))

	paths, err := yamlfmt.NewPatternFileCollectorFS(reader, tree).CollectPaths()
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("a pattern file that breaks while read collects %v with the error %v, want the reader's", paths, err)
	}
}
