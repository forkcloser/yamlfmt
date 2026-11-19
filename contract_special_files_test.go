//go:build unix

// What the collectors make of the files of a tree that are not regular.

package yamlfmt_test

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/forkcloser/yamlfmt"
)

// specialTree is a directory with, all named as YAML: a regular file, a
// link to a regular file outside the directory, a FIFO, a link to the
// FIFO, a link to a device and a link to a socket. It returns the
// directory.
func specialTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.yaml")

	//nolint:usetesting // a socket's path is at most some hundred bytes, and t.TempDir's holds the test's name
	short, err := os.MkdirTemp("", "s")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(short) })

	socket := filepath.Join(short, "s")

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	for _, file := range []string{filepath.Join(root, "regular.yaml"), outside} {
		if err := os.WriteFile(file, []byte("a: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := syscall.Mkfifo(filepath.Join(root, "fifo.yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	for link, target := range map[string]string{
		"link.yaml":     outside,
		"fifolink.yaml": filepath.Join(root, "fifo.yaml"),
		"device.yaml":   os.DevNull,
		"socket.yaml":   socket,
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// collected is the base names of what the collector returns, sorted.
func collected(t *testing.T, collector yamlfmt.PathCollector) []string {
	t.Helper()

	paths, err := collector.CollectPaths()
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}

	slices.Sort(names)

	return names
}

// TestContractCollectorsLeaveOutEndlessFiles: a file found in a directory,
// or by a pattern, is left out when it is a FIFO, a device or a socket,
// itself or through links: reading one ends when its writer likes, or
// never, or fails. A
// regular file is collected, itself or through a link. A path given by
// name is collected whatever it is.
func TestContractCollectorsLeaveOutEndlessFiles(t *testing.T) {
	t.Parallel()

	root := specialTree(t)
	fifo := filepath.Join(root, "fifo.yaml")

	cases := map[string]struct {
		collector yamlfmt.PathCollector
		want      []string
	}{
		"standard, a directory": {
			&yamlfmt.FilepathCollector{Include: []string{root}, Extensions: []string{"yaml"}},
			[]string{"link.yaml", "regular.yaml"},
		},
		"doublestar, a pattern": {
			&yamlfmt.DoublestarCollector{Include: []string{filepath.Join(root, "**", "*.yaml")}},
			[]string{"link.yaml", "regular.yaml"},
		},
		"standard, the FIFO by name": {
			&yamlfmt.FilepathCollector{Include: []string{fifo}, Extensions: []string{"yaml"}},
			[]string{"fifo.yaml"},
		},
		"doublestar, the FIFO by name": {
			&yamlfmt.DoublestarCollector{Include: []string{fifo}},
			[]string{"fifo.yaml"},
		},
	}

	for name, c := range cases {
		if got := collected(t, c.collector); !slices.Equal(got, c.want) {
			t.Errorf("%s: collected %v, want %v", name, got, c.want)
		}
	}
}

// TestContractPatternFileCollectsRegularFilesOnly: in the gitignore mode a
// pattern collects the regular files it matches and nothing else it
// matches: no FIFO, and no link, whatever it links to.
func TestContractPatternFileCollectsRegularFilesOnly(t *testing.T) {
	t.Parallel()

	collector := yamlfmt.NewPatternFileCollectorFS(strings.NewReader("*.yaml\n"), os.DirFS(specialTree(t)))

	if got, want := collected(t, collector), []string{"regular.yaml"}; !slices.Equal(got, want) {
		t.Errorf("collected %v, want %v", got, want)
	}
}
