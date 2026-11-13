package yaml_test

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// heapLimit is far above what these tests allocate and far below what the
// machine has.
const heapLimit = 4 << 30

// TestMain panics when the test binary's heap passes heapLimit, with every
// goroutine's stack. A parser or an emitter that stops making progress
// allocates until the machine has no memory left, which names no test and
// takes every other process with it.
func TestMain(m *testing.M) {
	go func() {
		var stats runtime.MemStats

		for {
			runtime.ReadMemStats(&stats)

			if stats.HeapAlloc > heapLimit {
				debug.SetTraceback("all")
				panic(fmt.Sprintf("the heap passed %d bytes: something allocates without end", uint64(heapLimit)))
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	m.Run()
}
