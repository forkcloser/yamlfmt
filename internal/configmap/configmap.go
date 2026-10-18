// Package configmap converts between the generic maps a YAML configuration
// decodes into and the structs that describe it, through the structs' yaml
// tags.
package configmap

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

// ErrNotStruct is returned when Decode is not given a pointer to a struct.
var ErrNotStruct = errors.New("configmap: not a pointer to a struct")

// Error lists the values that do not fit their fields, worded and laid out as
// the mapstructure configurations were decoded with reported them.
type Error struct {
	Problems []string
}

func (e *Error) Error() string {
	points := make([]string, len(e.Problems))
	for i, problem := range e.Problems {
		points[i] = "* " + problem
	}

	slices.Sort(points)

	return fmt.Sprintf("%d error(s) decoding:\n\n%s", len(points), strings.Join(points, "\n"))
}

// Decode fills the struct out points to from data. A key matches the field
// whose tag names it, regardless of case, an exact match first; a null value
// leaves its field as it was, so a default survives an empty setting; keys no
// field claims go to the struct's inline map, when it has one.
func Decode(data map[string]any, out any) error {
	target := reflect.TypeOf(out)
	if target == nil || target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("%w: %T", ErrNotStruct, out)
	}

	normalized, problems := normalize(data, target.Elem(), "")
	if len(problems) > 0 {
		return &Error{Problems: problems}
	}

	content, err := yaml.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("configmap: %w", err)
	}

	if err := yaml.Unmarshal(content, out); err != nil {
		return fmt.Errorf("configmap: %w", err)
	}

	return nil
}

// Map returns the fields of the struct in, or of the struct it points to, keyed
// by their tags, each value as the field holds it; omitempty fields at their
// zero value are left out.
func Map(in any) map[string]any {
	out := map[string]any{}

	for field, value := range reflect.Indirect(reflect.ValueOf(in)).Fields() {
		info, ok := tag(field)
		if !ok || info.inline || (info.omitempty && value.IsZero()) {
			continue
		}

		out[info.name] = value.Interface()
	}

	return out
}

// normalize renames data's keys to the tags of the struct type they match and
// drops the null ones, then does the same for every value that decodes into a
// nested struct.
func normalize(data map[string]any, structType reflect.Type, prefix string) (map[string]any, []string) {
	var problems []string

	out := make(map[string]any, len(data))
	maps.Copy(out, data)

	for field := range structType.Fields() {
		info, ok := tag(field)
		if !ok || info.inline {
			continue
		}

		key, found := match(out, info.name)
		if !found {
			continue
		}

		value := out[key]
		delete(out, key)

		if value == nil {
			continue
		}

		value, fieldProblems := normalizeValue(value, field.Type, prefix+info.name)
		problems = append(problems, fieldProblems...)
		out[info.name] = value
	}

	return out, problems
}

// normalizeValue prepares one field's value: a nested struct's map is
// normalized in turn, anything else is checked against the field's type. A
// value of the wrong type is a problem: YAML would turn 123 into "123" or yes
// into true, which a configuration given 123 for a path or yes for a flag never
// meant.
func normalizeValue(value any, fieldType reflect.Type, name string) (any, []string) {
	target := indirectType(fieldType)

	if nested, isMap := value.(map[string]any); isMap && target.Kind() == reflect.Struct {
		return normalize(nested, target, name+".")
	}

	if problems := check(value, fieldType, name); len(problems) > 0 {
		return value, problems
	}

	if items, isList := value.([]any); isList && target.Kind() == reflect.Slice {
		return zeroNulls(items, target.Elem()), nil
	}

	return value, nil
}

// check refuses a value its field's type cannot hold as it is: a string field
// takes a string, a bool field a bool, an integer field a number (a float
// truncates), a list a list of what its elements take, and a struct only the
// map normalizeValue already took.
func check(value any, fieldType reflect.Type, name string) []string {
	fieldType = indirectType(fieldType)
	valueType := reflect.TypeOf(value)

	var accepted bool

	switch fieldType.Kind() { //nolint:exhaustive // the kinds configuration fields have; YAML refuses the rest itself
	case reflect.String:
		accepted = valueType.Kind() == reflect.String
	case reflect.Bool:
		accepted = valueType.Kind() == reflect.Bool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		accepted = isNumber(valueType.Kind())
	case reflect.Struct:
		return []string{fmt.Sprintf("'%s' expected a map, got '%s'", name, valueType.Kind())}
	case reflect.Slice:
		items, isList := value.([]any)
		if !isList {
			return []string{fmt.Sprintf("'%s': source data must be an array or slice, got %s", name, valueType.Kind())}
		}

		var problems []string

		for i, item := range items {
			if item != nil {
				problems = append(problems, check(item, fieldType.Elem(), fmt.Sprintf("%s[%d]", name, i))...)
			}
		}

		return problems
	default:
		return nil
	}

	if accepted {
		return nil
	}

	return []string{fmt.Sprintf("'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
		name, fieldType, valueType, value)}
}

func isNumber(kind reflect.Kind) bool {
	switch kind { //nolint:exhaustive // the numeric kinds; every other kind is not a number
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// zeroNulls returns items with each null replaced by the element type's zero
// value: YAML would drop a null element from the list instead of keeping its
// place.
func zeroNulls(items []any, elemType reflect.Type) []any {
	out := make([]any, len(items))
	for i, item := range items {
		if item == nil {
			item = reflect.Zero(elemType).Interface()
		}

		out[i] = item
	}

	return out
}

// match finds the key of data that names the tag: the tag itself, else the
// first key equal to it under case folding, in sorted order.
func match(data map[string]any, name string) (string, bool) {
	if _, found := data[name]; found {
		return name, true
	}

	best, found := "", false

	for key := range data {
		if strings.EqualFold(key, name) && (!found || key < best) {
			best, found = key, true
		}
	}

	return best, found
}

type tagInfo struct {
	name      string
	omitempty bool
	inline    bool
}

// tag reads a field's yaml tag, and whether the field takes part at all.
func tag(field reflect.StructField) (tagInfo, bool) {
	if !field.IsExported() {
		return tagInfo{}, false
	}

	value, tagged := field.Tag.Lookup("yaml")
	if !tagged || value == "-" {
		return tagInfo{}, false
	}

	name, options, _ := strings.Cut(value, ",")
	info := tagInfo{
		name:      name,
		omitempty: strings.Contains(options, "omitempty"),
		inline:    strings.Contains(options, "inline"),
	}

	return info, name != "" || info.inline
}

func indirectType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}
