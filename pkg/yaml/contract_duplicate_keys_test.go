// What decoding a mapping into a Go value costs in its keys: the path a
// configuration file takes.

package yaml_test

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

// leastTime is the shortest of five runs of f: the one the rest of the
// machine disturbed least.
func leastTime(f func()) time.Duration {
	least := time.Duration(math.MaxInt64)

	for range 5 {
		start := time.Now()

		f()

		least = min(least, time.Since(start))
	}

	return least
}

// TestContractDuplicateKeysAreOneErrorEach: a mapping key written again is
// one error, on its own line, naming the line of the key's first
// definition, in the order the keys are written. A key written n times is
// n-1 errors, so the errors of a mapping are no more than its keys, and
// what the decoder allocates for them a small multiple of the input.
//
//nolint:paralleltest // measures what the process allocates
func TestContractDuplicateKeysAreOneErrorEach(t *testing.T) {
	var out map[string]any

	err := yaml.Unmarshal([]byte("a: 1\nb: 2\nb: 3\na: 4\nb: 5\n"), &out)

	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("want a TypeError, got %v", err)
	}

	want := []string{
		`line 3: mapping key "b" already defined at line 2`,
		`line 4: mapping key "a" already defined at line 1`,
		`line 5: mapping key "b" already defined at line 2`,
	}
	if strings.Join(typeErr.Errors, "\n") != strings.Join(want, "\n") {
		t.Errorf("errors:\n%s\nwant:\n%s", strings.Join(typeErr.Errors, "\n"), strings.Join(want, "\n"))
	}

	// 2500 bytes: one error for each pair of them would be 124750 errors
	// and 6 MiB of text.
	const (
		keys       = 500
		allocation = 2 << 20
	)

	input := []byte(strings.Repeat("a: 1\n", keys))

	var before, after runtime.MemStats

	runtime.ReadMemStats(&before)

	err = yaml.Unmarshal(input, &out)

	runtime.ReadMemStats(&after)

	if !errors.As(err, &typeErr) {
		t.Fatalf("want a TypeError, got %v", err)
	}

	if len(typeErr.Errors) != keys-1 {
		t.Errorf("a key written %d times is %d errors, want %d", keys, len(typeErr.Errors), keys-1)
	}

	if spent := after.TotalAlloc - before.TotalAlloc; spent > allocation {
		t.Errorf("decoding %d bytes allocated %d bytes, over %d", len(input), spent, allocation)
	}
}

// TestContractDistinctKeysAreCheckedOnce: the check that a mapping's keys
// are distinct takes time in proportion to the keys, not to their pairs:
// four times the keys do not take ten times as long (sixteen, by pairs).
//
//nolint:paralleltest // measures time
func TestContractDistinctKeysAreCheckedOnce(t *testing.T) {
	mapping := func(keys int) []byte {
		var b strings.Builder

		for i := range keys {
			fmt.Fprintf(&b, "k%d: 1\n", i)
		}

		return []byte(b.String())
	}

	decode := func(input []byte) func() {
		return func() {
			var out map[string]any

			if err := yaml.Unmarshal(input, &out); err != nil {
				t.Fatal(err)
			}
		}
	}

	const keys = 8000

	small := leastTime(decode(mapping(keys)))
	large := leastTime(decode(mapping(4 * keys)))

	t.Logf("%d keys take %v, %d keys take %v", keys, small, 4*keys, large)

	if large > 10*small {
		t.Errorf("four times the keys take %.1f times as long", float64(large)/float64(small))
	}
}
