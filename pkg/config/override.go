package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// This file is the programmatic write side of config-override.yaml, shared by
// `tacit config set|unset` and the Mac app's settings screen.
//
// The override file is also edited by hand (`tacit config edit`), so it is
// edited here as text rather than round-tripped through a YAML encoder, which
// would reformat the file and drop the user's comments. Every edit is then
// checked by parsing: an edit that would leave the file unloadable, or change
// any key other than the one asked for, is refused and nothing is written.

// Field is one setting as a settings screen shows it.
type Field struct {
	Key string `json:"key"`
	// Value is the effective setting. Durations are strings in the form the
	// files use ("5s", "3h"), so a value read here can be written straight back.
	Value any `json:"value"`
	// Default is what the setting would be with no override.
	Default any `json:"default"`
	// Overridden reports whether config-override.yaml sets this key.
	Overridden bool `json:"overridden"`
	// Kind is the value's type, for choosing an input: "string", "bool",
	// "number", "duration" (a string with a unit), or "list" (of strings).
	Kind string `json:"kind"`
}

var durationType = reflect.TypeOf(time.Duration(0))

// Keys returns every config key, in the order Config declares them.
func Keys() []string {
	t := reflect.TypeOf(Config{})
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if k := yamlKey(t.Field(i)); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

func yamlKey(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
	if name == "-" {
		return ""
	}
	return name
}

// fieldByKey returns the Config struct field for a yaml key.
func fieldByKey(key string) (reflect.StructField, bool) {
	t := reflect.TypeOf(Config{})
	for i := 0; i < t.NumField(); i++ {
		if yamlKey(t.Field(i)) == key {
			return t.Field(i), true
		}
	}
	return reflect.StructField{}, false
}

// Fields reports every setting with its effective value, its default, and
// whether the override file sets it.
func Fields(configPath, overridePath string) ([]Field, error) {
	effective, err := LoadWithOverride(configPath, overridePath)
	if err != nil {
		return nil, err
	}
	defaults, err := LoadWithOverride(configPath, "")
	if err != nil {
		return nil, err
	}
	overridden, err := LoadOverrideKeys(overridePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", overridePath, err)
	}

	ev, dv := reflect.ValueOf(effective).Elem(), reflect.ValueOf(defaults).Elem()
	var out []Field
	for _, key := range Keys() {
		f, _ := fieldByKey(key)
		out = append(out, Field{
			Key:        key,
			Value:      displayValue(ev.FieldByIndex(f.Index)),
			Default:    displayValue(dv.FieldByIndex(f.Index)),
			Overridden: overridden[key],
			Kind:       kindOf(f.Type),
		})
	}
	return out, nil
}

func kindOf(t reflect.Type) string {
	switch {
	case t == durationType:
		return "duration"
	case t.Kind() == reflect.Bool:
		return "bool"
	case t.Kind() == reflect.Float64:
		return "number"
	case t.Kind() == reflect.Slice:
		return "list"
	}
	return "string"
}

// displayValue renders a Config field for a settings screen: durations in
// file form, and lists as [] rather than null when empty.
func displayValue(v reflect.Value) any {
	switch {
	case v.Type() == durationType:
		return formatDuration(time.Duration(v.Int()))
	case v.Kind() == reflect.Slice && v.IsNil():
		return reflect.MakeSlice(v.Type(), 0, 0).Interface()
	}
	return v.Interface()
}

// SetOverride sets key to value in the override file at path, creating the
// file from the commented template if it does not exist. value is a decoded
// YAML or JSON value: a string, bool, number, or list of strings. Durations
// are strings with a unit ("30s").
//
// The key's existing line is replaced in place, or its commented-out template
// line is uncommented, so the file keeps its order and every other line.
func SetOverride(path, key string, value any) error {
	f, ok := fieldByKey(key)
	if !ok {
		return fmt.Errorf("unknown config key %q", key)
	}
	value, err := normalizeDuration(f, value)
	if err != nil {
		return err
	}
	rendered, err := renderValue(value)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return editOverride(path, key, key+": "+rendered, true)
}

// ParseValue converts text typed for key, as on a command line, into a value
// SetOverride accepts. String settings take the text verbatim, so a prompt
// like "tacit: whisper" is not read as a YAML mapping. List settings accept a
// YAML list ("[a, b]") or a single bare item. Everything else is parsed as a
// YAML value, which is what makes "false" a bool and "0.6" a number.
func ParseValue(key, text string) (any, error) {
	f, ok := fieldByKey(key)
	if !ok {
		return nil, fmt.Errorf("unknown config key %q", key)
	}
	if f.Type.Kind() == reflect.String {
		return text, nil
	}
	var v any
	if err := yaml.Unmarshal([]byte(text), &v); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	if f.Type.Kind() == reflect.Slice {
		switch v.(type) {
		case []any:
		case nil:
			v = []any{}
		default:
			v = []any{text}
		}
	}
	return v, nil
}

// ClearOverride removes key from the override file at path, so the default
// applies again. The line is replaced by the key's commented-out line from the
// template, so a cleared key reads exactly as it would in a fresh file.
// Clearing a key the file does not set is a no-op.
func ClearOverride(path, key string) error {
	if _, ok := fieldByKey(key); !ok {
		return fmt.Errorf("unknown config key %q", key)
	}
	line, ok := templateLine(key)
	if !ok {
		return fmt.Errorf("no template line for %q", key)
	}
	return editOverride(path, key, line, false)
}

// templateLine returns the commented-out line the override template has for key.
func templateLine(key string) (string, bool) {
	prefix := "# " + key + ":"
	for _, l := range strings.Split(overrideTemplate(DefaultConfig()), "\n") {
		if strings.HasPrefix(l, prefix) {
			return l, true
		}
	}
	return "", false
}

// normalizeDuration handles a number given for a duration. YAML cannot decode
// a bare number into one — `max_segment_duration: 30` makes the whole file
// fail to load — so 0, which means the same with or without a unit, becomes
// "0s", and any other number is refused with the unit it was probably meant
// to have.
func normalizeDuration(f reflect.StructField, value any) (any, error) {
	if f.Type != durationType {
		return value, nil
	}
	var n float64
	switch v := value.(type) {
	case int:
		n = float64(v)
	case float64:
		n = v
	default:
		return value, nil
	}
	if n == 0 {
		return "0s", nil
	}
	return nil, fmt.Errorf("%s needs a unit, e.g. \"%gs\"", yamlKey(f), n)
}

// renderValue renders value as a single-line YAML value: lists in flow style,
// and strings quoted wherever YAML would otherwise read them as something else.
func renderValue(value any) (string, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return "", fmt.Errorf("encoding value: %w", err)
	}
	flatten(&node)
	out, err := yaml.Marshal(&node)
	if err != nil {
		return "", fmt.Errorf("encoding value: %w", err)
	}
	s := strings.TrimSuffix(string(out), "\n")
	if strings.Contains(s, "\n") {
		return "", fmt.Errorf("value does not fit on one line: %q", s)
	}
	return s, nil
}

// flatten forces a node onto one line: flow style for collections, and double
// quotes for any string containing a newline.
func flatten(n *yaml.Node) {
	switch n.Kind {
	case yaml.SequenceNode, yaml.MappingNode:
		n.Style = yaml.FlowStyle
	case yaml.ScalarNode:
		if strings.ContainsAny(n.Value, "\n\r") {
			n.Style = yaml.DoubleQuotedStyle
		}
	}
	for _, c := range n.Content {
		flatten(c)
	}
}

// editOverride rewrites the line (or block) for key in the override file to
// newLine, verifies the result, and writes it atomically. set says whether the
// key should be present after the edit.
func editOverride(path, key, newLine string, set bool) error {
	before, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if !set {
			return nil // nothing overrides the key, so it is already cleared
		}
		before = []byte(overrideTemplate(DefaultConfig()))
	case err != nil:
		return fmt.Errorf("reading %s: %w", path, err)
	}

	beforeMap := map[string]any{}
	if err := yaml.Unmarshal(before, &beforeMap); err != nil {
		return fmt.Errorf("%s is not valid YAML, fix it with 'tacit config edit' first: %w", path, err)
	}
	if _, present := beforeMap[key]; !set && !present {
		return nil
	}

	after := []byte(replaceKeyLines(string(before), key, newLine, set))

	// Verify before writing: the file must still load into Config, and no key
	// other than this one may have changed.
	if err := yaml.Unmarshal(after, DefaultConfig()); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	afterMap := map[string]any{}
	if err := yaml.Unmarshal(after, &afterMap); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if _, present := afterMap[key]; present != set {
		if set {
			return fmt.Errorf("editing %s did not set %q; refusing to write", path, key)
		}
		return fmt.Errorf("editing %s did not clear %q; refusing to write", path, key)
	}
	for k := range union(beforeMap, afterMap) {
		if k != key && !reflect.DeepEqual(beforeMap[k], afterMap[k]) {
			return fmt.Errorf("editing %q would also change %q; refusing to write %s", key, k, path)
		}
	}

	return writeAtomic(path, after)
}

func union(a, b map[string]any) map[string]struct{} {
	out := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

// replaceKeyLines returns content with key's line replaced by newLine. An
// active key is replaced together with any block value under it (an indented
// or "- " list on the following lines). Otherwise, when setting, the first
// commented-out "# key:" line is replaced, and failing that newLine is appended.
func replaceKeyLines(content, key, newLine string, set bool) string {
	lines := strings.Split(content, "\n")
	active := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*:`)
	commented := regexp.MustCompile(`^#\s*` + regexp.QuoteMeta(key) + `\s*:`)

	for i, l := range lines {
		if !active.MatchString(l) {
			continue
		}
		end := i + 1
		for end < len(lines) && isContinuation(lines, end) {
			end++
		}
		return strings.Join(append(append(lines[:i:i], newLine), lines[end:]...), "\n")
	}
	if !set {
		return content
	}
	for i, l := range lines {
		if commented.MatchString(l) {
			lines[i] = newLine
			return strings.Join(lines, "\n")
		}
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + newLine + "\n"
}

// isContinuation reports whether lines[i] belongs to the block value of the
// key above it: indented, a top-level "- " list item, or a blank line that
// such a line follows.
func isContinuation(lines []string, i int) bool {
	l := lines[i]
	if strings.TrimSpace(l) == "" {
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) != "" {
				return isContinuation(lines, j)
			}
		}
		return false
	}
	return l[0] == ' ' || l[0] == '\t' || l == "-" || strings.HasPrefix(l, "- ")
}

// writeAtomic replaces path with data via a temp file in the same directory,
// so a reader never sees a half-written config.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-override-*.yaml")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
