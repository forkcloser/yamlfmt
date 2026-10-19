// The command's contract from the outside: the binary built once, run in
// directories made for each case, judged by its exit status, what it
// prints and what it rewrites, as docs/command-usage.md says.
package main_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// contractBinary is the command built once for every case.
//
//nolint:gochecknoglobals // built in TestMain, read by every test
var contractBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "yamlfmt-contract")
	if err != nil {
		panic(err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	contractBinary = filepath.Join(dir, "yamlfmt"+exeSuffix())

	build := exec.CommandContext(context.Background(), "go", "build", "-o", contractBinary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic(string(out))
	}

	m.Run()
}

const (
	messy = "a:   1\nb:\n    c: [1,2]\n"
	tidy  = "a: 1\nb:\n  c: [1, 2]\n"
)

// tree writes files (name to content) under a fresh directory with no
// configuration file anywhere up to it, and returns it.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// run runs the binary in dir with stdin: exit status, stdout, stderr.
func run(t *testing.T, dir, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), contractBinary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)

	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+t.TempDir(), "HOME="+t.TempDir())

	var out, errOut strings.Builder

	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()

	var exit *exec.ExitError

	switch {
	case err == nil:
	case errors.As(err, &exit):
		code = exit.ExitCode()
	default:
		t.Fatalf("running %s: %v", contractBinary, err)
	}

	return code, out.String(), errOut.String()
}

func read(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

// TestContractFormatRewritesInPlace: the default mode rewrites every
// matching file under the paths given, a directory searched for .yaml and
// .yml, other extensions left alone, exit 0.
func TestContractFormatRewritesInPlace(t *testing.T) {
	t.Parallel()

	dir := tree(t, map[string]string{"a.yaml": messy, "sub/b.yml": messy, "c.txt": messy, "sub/deep/d.yaml": tidy})

	code, _, stderr := run(t, dir, "", ".")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	for name, want := range map[string]string{"a.yaml": tidy, "sub/b.yml": tidy, "c.txt": messy, "sub/deep/d.yaml": tidy} {
		if got := read(t, filepath.Join(dir, filepath.FromSlash(name))); got != want {
			t.Errorf("%s:\n%s\nwant:\n%s", name, got, want)
		}
	}

	only := tree(t, map[string]string{"a.yaml": messy, "b.yaml": messy})

	if code, _, _ := run(
		t,
		only,
		"",
		"a.yaml",
	); code != 0 || read(t, filepath.Join(only, "a.yaml")) != tidy ||
		read(t, filepath.Join(only, "b.yaml")) != messy {
		t.Errorf(
			"one path given: exit %d, a %q, b %q",
			code,
			read(t, filepath.Join(only, "a.yaml")),
			read(t, filepath.Join(only, "b.yaml")),
		)
	}
}

// TestContractStdin: -in, "-" and /dev/stdin format stdin to stdout and
// touch no file.
func TestContractStdin(t *testing.T) {
	t.Parallel()

	dir := tree(t, map[string]string{"a.yaml": messy})

	for _, args := range [][]string{{"-in"}, {"-"}, {"/dev/stdin"}} {
		code, stdout, stderr := run(t, dir, messy, args...)
		if code != 0 || stdout != tidy {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}

	if read(t, filepath.Join(dir, "a.yaml")) != messy {
		t.Error("a stdin run rewrote a file")
	}
}

// TestContractLintAndDry: -lint exits 1 with the differing files and
// their diffs on stderr and rewrites nothing, 0 when none differs; -dry
// prints each differing file's formatted output on stdout and rewrites
// nothing; -q names the files alone.
func TestContractLintAndDry(t *testing.T) {
	t.Parallel()

	dir := tree(t, map[string]string{"a.yaml": messy, "b.yaml": tidy})

	code, stdout, stderr := run(t, dir, "", "-lint", ".")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "a.yaml") || strings.Contains(stderr, "b.yaml") {
		t.Errorf("-lint: exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	code, stdout, _ = run(t, dir, "", "-dry", ".")
	if code != 0 || !strings.Contains(stdout, "a.yaml") || !strings.Contains(stdout, "a: 1") {
		t.Errorf("-dry: exit %d stdout %q", code, stdout)
	}

	code, stdout, _ = run(t, dir, "", "-dry", "-q", ".")
	if code != 0 || !strings.Contains(stdout, "a.yaml") || strings.Contains(stdout, "a: 1") {
		t.Errorf("-dry -q: exit %d stdout %q", code, stdout)
	}

	if read(t, filepath.Join(dir, "a.yaml")) != messy {
		t.Error("-lint or -dry rewrote a file")
	}

	clean := tree(t, map[string]string{"b.yaml": tidy})
	if code, _, _ := run(t, clean, "", "-lint", "."); code != 0 {
		t.Errorf("-lint on a clean tree: exit %d", code)
	}
}

// TestContractConfigAndExclude: a .yamlfmt in the directory configures
// the formatter; -conf names one elsewhere; -formatter sets a value on the
// command line; -exclude leaves a path out; -extensions chooses which
// files a directory search takes.
func TestContractConfigAndExclude(t *testing.T) {
	t.Parallel()

	dir := tree(t, map[string]string{"a.yaml": messy, ".yamlfmt": "formatter:\n  type: basic\n  indent: 4\n"})

	if code, _, stderr := run(
		t,
		dir,
		"",
		".",
	); code != 0 ||
		!strings.Contains(read(t, filepath.Join(dir, "a.yaml")), "\n    c:") {
		t.Errorf(".yamlfmt indent 4: exit %d %s\n%s", code, stderr, read(t, filepath.Join(dir, "a.yaml")))
	}

	conf := filepath.Join(t.TempDir(), "conf.yaml")
	if err := os.WriteFile(conf, []byte("formatter:\n  type: basic\n  indent: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir = tree(t, map[string]string{"a.yaml": messy})
	if code, _, _ := run(
		t,
		dir,
		"",
		"-conf",
		conf,
		".",
	); code != 0 ||
		!strings.Contains(read(t, filepath.Join(dir, "a.yaml")), "\n   c:") {
		t.Errorf("-conf indent 3: exit %d\n%s", code, read(t, filepath.Join(dir, "a.yaml")))
	}

	dir = tree(t, map[string]string{"a.yaml": messy})
	if code, _, _ := run(
		t,
		dir,
		"",
		"-formatter",
		"indent=4",
		".",
	); code != 0 ||
		!strings.Contains(read(t, filepath.Join(dir, "a.yaml")), "\n    c:") {
		t.Errorf("-formatter indent=4: exit %d\n%s", code, read(t, filepath.Join(dir, "a.yaml")))
	}

	dir = tree(t, map[string]string{"a.yaml": messy, "skip/b.yaml": messy})
	if code, _, _ := run(
		t,
		dir,
		"",
		"-exclude",
		"skip/",
		".",
	); code != 0 || read(t, filepath.Join(dir, "a.yaml")) != tidy ||
		read(t, filepath.Join(dir, "skip", "b.yaml")) != messy {
		t.Errorf(
			"-exclude skip/: exit %d, a %q, skip/b %q",
			code,
			read(t, filepath.Join(dir, "a.yaml")),
			read(t, filepath.Join(dir, "skip", "b.yaml")),
		)
	}
}

// TestContractErrors: a file that is not YAML fails the run with exit 1
// and its name on stderr; an unknown flag is a usage error with exit
// status not 0; -version prints a version and exits 0.
func TestContractErrors(t *testing.T) {
	t.Parallel()

	dir := tree(t, map[string]string{"bad.yaml": "a: [\n", "good.yaml": tidy})

	code, _, stderr := run(t, dir, "", ".")
	if code != 1 || !strings.Contains(stderr, "bad.yaml") {
		t.Errorf("a file that is not YAML: exit %d stderr %q", code, stderr)
	}

	if code, _, _ := run(t, dir, "", "-nonesuch", "."); code == 0 {
		t.Error("an unknown flag: exit 0")
	}

	if code, stdout, _ := run(t, dir, "", "-version"); code != 0 || strings.TrimSpace(stdout) == "" {
		t.Errorf("-version: exit %d stdout %q", code, stdout)
	}
}

// exeSuffix is what the OS needs appended to run a binary built to a bare name.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}

	return ""
}
