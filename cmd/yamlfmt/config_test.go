// Copyright 2026 Forkcloser.
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

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestValidatePath: a directory is refused as one, a path that cannot be
// stat'ed for another reason than absence is an error and not a panic, a
// missing path is reported as missing, and a file is accepted.
func TestValidatePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := validatePath(dir); !errors.Is(err, errConfPathIsDir) {
		t.Errorf("a directory: %v; want errConfPathIsDir", err)
	}

	file := filepath.Join(dir, ".yamlfmt")
	if err := os.WriteFile(file, []byte("indent: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := validatePath(file); err != nil {
		t.Errorf("a file: %v", err)
	}

	if err := validatePath(filepath.Join(dir, "missing")); !errors.Is(err, errConfPathNotExist) {
		t.Errorf("a missing path: %v; want errConfPathNotExist", err)
	}

	// a path through a regular file: the stat fails. Unix says the parent
	// is not a directory, which is another error than absence; Windows
	// says the path is not found, which is absence.
	err := validatePath(filepath.Join(file, ".yamlfmt"))
	if runtime.GOOS == "windows" {
		if !errors.Is(err, errConfPathNotExist) {
			t.Errorf("a path through a file: %v; want errConfPathNotExist", err)
		}
	} else if err == nil || errors.Is(err, errConfPathNotExist) {
		t.Errorf("a path through a file: %v; want another error", err)
	}
}

// TestConfigPathPrecedence: with several configuration files present, the
// first name in the documented order wins, every time.
func TestConfigPathPrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for _, name := range []string{"yamlfmt.yaml", ".yamlfmt.yml", ".yamlfmt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("indent: 2\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for range 20 {
		got, err := getConfigPathFromDir(dir)
		if err != nil || filepath.Base(got) != ".yamlfmt" {
			t.Fatalf("%q, %v; want .yamlfmt", got, err)
		}
	}

	if err := os.Remove(filepath.Join(dir, ".yamlfmt")); err != nil {
		t.Fatal(err)
	}

	if got, err := getConfigPathFromDir(dir); err != nil || filepath.Base(got) != "yamlfmt.yaml" {
		t.Fatalf("%q, %v; want yamlfmt.yaml once .yamlfmt is gone", got, err)
	}
}
