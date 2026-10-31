// Copyright 2026 Forkcloser
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yamlfmt_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/forkcloser/yamlfmt"
)

// unreadableTree is a directory holding one readable yaml file and one
// subdirectory nobody may list.
func unreadableTree(t *testing.T) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not deny listing on Windows")
	}

	if os.Geteuid() == 0 {
		t.Skip("root lists any directory")
	}

	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "a.yaml"), []byte("a: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	closed := filepath.Join(root, "closed")
	if err := os.Mkdir(closed, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(closed, "b.yaml"), []byte("b: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(closed, 0o700) })

	return root
}

// TestCollectorsFailOnAnUnreadableDirectory: a directory the walk cannot read
// holds files that would be neither checked nor formatted, so collecting
// fails instead of returning the paths it did reach.
func TestCollectorsFailOnAnUnreadableDirectory(t *testing.T) {
	t.Parallel()

	root := unreadableTree(t)

	for name, collector := range map[string]yamlfmt.PathCollector{
		"filepath":   &yamlfmt.FilepathCollector{Include: []string{root}, Extensions: []string{"yaml"}},
		"doublestar": &yamlfmt.DoublestarCollector{Include: []string{filepath.Join(root, "**", "*.yaml")}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			paths, err := collector.CollectPaths()
			if err == nil {
				t.Fatalf("CollectPaths returned %v and no error", paths)
			}
		})
	}
}
