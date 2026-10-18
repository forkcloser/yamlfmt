package configmap_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/forkcloser/yamlfmt/internal/configmap"
	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

type style string

type nested struct {
	Kind  string         `yaml:"kind"`
	Extra map[string]any `yaml:",inline"`
}

type config struct {
	Indent int      `yaml:"indent"`
	Strict bool     `yaml:"strict"`
	Style  style    `yaml:"style"`
	Paths  []string `yaml:"paths"`
	Nested *nested  `yaml:"nested,omitempty"`
}

func defaults() *config {
	return &config{Indent: 2, Style: "plain", Nested: &nested{Extra: map[string]any{}}}
}

func decode(t *testing.T, document string) (*config, error) {
	t.Helper()

	var data map[string]any
	if err := yaml.Unmarshal([]byte(document), &data); err != nil {
		t.Fatalf("%q: %v", document, err)
	}

	cfg := defaults()

	return cfg, configmap.Decode(data, cfg)
}

func TestDecode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		document string
		want     *config
	}{
		{"empty", "", defaults()},
		{"values", "indent: 4\nstrict: true\nstyle: fancy\npaths: [a, b]", &config{
			Indent: 4, Strict: true, Style: "fancy", Paths: []string{"a", "b"}, Nested: &nested{Extra: map[string]any{}},
		}},
		{"a key matches its tag regardless of case", "Indent: 4\nSTRICT: true", &config{
			Indent: 4, Strict: true, Style: "plain", Nested: &nested{Extra: map[string]any{}},
		}},
		{"the exact key wins over a folded one", "INDENT: 6\nindent: 4", &config{
			Indent: 4, Style: "plain", Nested: &nested{Extra: map[string]any{}},
		}},
		{"a null leaves the default", "indent:\nstyle: null\nnested: ~", defaults()},
		{"a float truncates into an integer", "indent: 4.7", &config{
			Indent: 4, Style: "plain", Nested: &nested{Extra: map[string]any{}},
		}},
		{"a null element keeps its place", "paths: [a, ~, b]", &config{
			Indent: 2, Style: "plain", Paths: []string{"a", "", "b"}, Nested: &nested{Extra: map[string]any{}},
		}},
		{"unclaimed keys go to the inline map, folded keys to their field", "nested: {Kind: k, other: 1, Deep: {X: 1}}", &config{
			Indent: 2, Style: "plain", Nested: &nested{Kind: "k", Extra: map[string]any{"other": 1, "Deep": map[string]any{"X": 1}}},
		}},
		{"unknown keys are ignored", "unknown: 1", defaults()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decode(t, tc.document)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v (nested %+v), want %+v (nested %+v)", got, got.Nested, tc.want, tc.want.Nested)
			}
		})
	}
}

func TestDecodeRefusesTheWrongType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		document string
		want     string
	}{
		{"strict: yes", "'strict' expected type 'bool', got unconvertible type 'string', value: 'yes'"},
		{"strict: 1", "'strict' expected type 'bool', got unconvertible type 'int'"},
		{"style: 123", "'style' expected type 'configmap_test.style', got unconvertible type 'int'"},
		{"indent: '4'", "'indent' expected type 'int', got unconvertible type 'string'"},
		{"paths: a", "'paths': source data must be an array or slice, got string"},
		{"paths: [a, 1]", "'paths[1]' expected type 'string', got unconvertible type 'int'"},
		{"nested: {kind: true}", "'nested.kind' expected type 'string', got unconvertible type 'bool'"},
		{"nested: [a]", "'nested' expected a map, got 'slice'"},
		{"indent: 99999999999999999999", "cannot unmarshal !!float"},
	} {
		t.Run(tc.document, func(t *testing.T) {
			t.Parallel()

			_, err := decode(t, tc.document)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestDecodeRefusesANonStruct(t *testing.T) {
	t.Parallel()

	var notAStruct map[string]any
	if err := configmap.Decode(map[string]any{}, &notAStruct); err == nil {
		t.Fatal("decoding into a map succeeded")
	}
}

func TestMap(t *testing.T) {
	t.Parallel()

	cfg := &config{Indent: 4, Style: "fancy", Paths: []string{"a"}}

	want := map[string]any{"indent": 4, "strict": false, "style": style("fancy"), "paths": []string{"a"}}
	if got := configmap.Map(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	cfg.Nested = &nested{Kind: "k"}
	if got := configmap.Map(*cfg); got["nested"] != cfg.Nested {
		t.Errorf("a set omitempty field is kept as it is: got %#v", got["nested"])
	}
}

func TestMapLeavesTheInlineMapOut(t *testing.T) {
	t.Parallel()

	got := configmap.Map(&nested{Kind: "k", Extra: map[string]any{"indent": 4}})

	want := map[string]any{"kind": "k"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestDecodeErrorLayout(t *testing.T) {
	t.Parallel()

	_, err := decode(t, "strict: 1\npaths: [a, 2]")

	want := "2 error(s) decoding:\n\n" +
		"* 'paths[1]' expected type 'string', got unconvertible type 'int', value: '2'\n" +
		"* 'strict' expected type 'bool', got unconvertible type 'int', value: '1'"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}
