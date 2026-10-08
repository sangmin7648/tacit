package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func overridePathIn(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config-override.yaml")
}

func writeOverride(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing override: %v", err)
	}
}

func readOverride(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading override: %v", err)
	}
	return string(b)
}

func loadOverride(t *testing.T, path string) *Config {
	t.Helper()
	cfg, err := LoadWithOverride("", path)
	if err != nil {
		t.Fatalf("loading override: %v", err)
	}
	return cfg
}

// With no override file yet, a set has to leave behind the same commented
// template `tacit config edit` would have created, with just that key live.
func TestSetOverride_CreatesFileFromTemplate(t *testing.T) {
	path := overridePathIn(t)

	if err := SetOverride(path, "language", "ko"); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}

	got := readOverride(t, path)
	want := strings.Replace(overrideTemplate(DefaultConfig()), "# language: auto\n", "language: ko\n", 1)
	if got != want {
		t.Errorf("file =\n%s\nwant =\n%s", got, want)
	}
	if cfg := loadOverride(t, path); cfg.Language != "ko" {
		t.Errorf("Language = %q, want ko", cfg.Language)
	}
}

// Uncommenting in place keeps the file's order: the user finds the key where
// the template put it, not appended at the bottom.
func TestSetOverride_UncommentsTemplateLineInPlace(t *testing.T) {
	path := overridePathIn(t)
	if err := WriteOverrideTemplate(path, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	before := strings.Split(readOverride(t, path), "\n")

	if err := SetOverride(path, "silence_duration", "8s"); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}

	after := strings.Split(readOverride(t, path), "\n")
	if len(after) != len(before) {
		t.Fatalf("line count %d -> %d; want an in-place edit", len(before), len(after))
	}
	for i := range before {
		if strings.HasPrefix(before[i], "# silence_duration:") {
			if after[i] != "silence_duration: 8s" {
				t.Errorf("line %d = %q, want %q", i, after[i], "silence_duration: 8s")
			}
			continue
		}
		if after[i] != before[i] {
			t.Errorf("line %d changed: %q -> %q", i, before[i], after[i])
		}
	}
}

// The point of editing text instead of re-encoding: a hand-edited file keeps
// every byte the edit was not about, including the user's own comments.
func TestSetOverride_PreservesUnrelatedBytes(t *testing.T) {
	path := overridePathIn(t)
	const orig = "# my settings — don't touch\n" +
		"language: en   # pinned for meetings\n" +
		"\n" +
		"# model notes: turbo is fast enough\n" +
		"llm_provider: claude\n" +
		"experimental: true\n"
	writeOverride(t, path, orig)

	if err := SetOverride(path, "llm_provider", "ollama"); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}

	want := strings.Replace(orig, "llm_provider: claude\n", "llm_provider: ollama\n", 1)
	if got := readOverride(t, path); got != want {
		t.Errorf("file =\n%s\nwant =\n%s", got, want)
	}
}

// A block list spans several lines. Replacing only the key line would orphan
// the items and break the file.
func TestSetOverride_ReplacesBlockList(t *testing.T) {
	for name, block := range map[string]string{
		"indented":   "transcript_denylist:\n  - old one\n  - old two\n",
		"unindented": "transcript_denylist:\n- old one\n- old two\n",
		"with blank": "transcript_denylist:\n  - old one\n\n  - old two\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := overridePathIn(t)
			writeOverride(t, path, "language: ko\n"+block+"experimental: true\n")

			if err := SetOverride(path, "transcript_denylist", []any{"시청해주셔서 감사합니다", "new"}); err != nil {
				t.Fatalf("SetOverride: %v", err)
			}

			want := "language: ko\ntranscript_denylist: [시청해주셔서 감사합니다, new]\nexperimental: true\n"
			if got := readOverride(t, path); got != want {
				t.Errorf("file =\n%s\nwant =\n%s", got, want)
			}
			cfg := loadOverride(t, path)
			if !reflect.DeepEqual(cfg.TranscriptDenylist, []string{"시청해주셔서 감사합니다", "new"}) {
				t.Errorf("TranscriptDenylist = %q", cfg.TranscriptDenylist)
			}
			if !cfg.Experimental {
				t.Error("experimental lost its override")
			}
		})
	}
}

// Values have to come back exactly as set, including strings YAML would
// otherwise read as another type and strings with newlines.
func TestSetOverride_RoundTripsAwkwardValues(t *testing.T) {
	cases := []struct {
		key   string
		value any
		check func(*Config) any
		want  any
	}{
		{"initial_prompt", "yes", func(c *Config) any { return c.InitialPrompt }, "yes"},
		{"initial_prompt", "tacit: whisper, VAD # not a comment", func(c *Config) any { return c.InitialPrompt }, "tacit: whisper, VAD # not a comment"},
		{"initial_prompt", "line one\nline two", func(c *Config) any { return c.InitialPrompt }, "line one\nline two"},
		{"initial_prompt", "", func(c *Config) any { return c.InitialPrompt }, ""},
		{"llm_model", "3.5", func(c *Config) any { return c.LLMModel }, "3.5"},
		{"experimental", true, func(c *Config) any { return c.Experimental }, true},
		{"speech_threshold", 0.65, func(c *Config) any { return c.SpeechThreshold }, 0.65},
		{"energy_threshold", 300, func(c *Config) any { return c.EnergyThreshold }, 300.0},
		{"dedup_window", "90m", func(c *Config) any { return c.DedupWindow }, 90 * time.Minute},
		{"max_segment_duration", 0, func(c *Config) any { return c.MaxSegmentDur }, time.Duration(0)},
		{"transcript_denylist", []any{}, func(c *Config) any { return len(c.TranscriptDenylist) }, 0},
	}
	for _, c := range cases {
		path := overridePathIn(t)
		if err := SetOverride(path, c.key, c.value); err != nil {
			t.Errorf("SetOverride(%s, %#v): %v", c.key, c.value, err)
			continue
		}
		if got := c.check(loadOverride(t, path)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %#v after setting %#v, want %#v", c.key, got, c.value, c.want)
		}
	}
}

// A refused edit must leave the file exactly as it was.
func TestSetOverride_RefusesAndLeavesFileUntouched(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value any
		want  string // substring of the error
	}{
		{"unknown key", "whisper_modle", "base", "unknown config key"},
		{"wrong type", "experimental", "maybe", "experimental"},
		{"list for scalar", "language", []any{"ko", "en"}, "language"},
		{"bare int duration", "silence_duration", 30, "needs a unit"},
		{"bare float duration", "silence_duration", 1.5, "needs a unit"},
		{"unparseable duration", "silence_duration", "soon", "silence_duration"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := overridePathIn(t)
			const orig = "language: en\n# silence_duration: 3s\n"
			writeOverride(t, path, orig)

			err := SetOverride(path, c.key, c.value)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one mentioning %q", err, c.want)
			}
			if got := readOverride(t, path); got != orig {
				t.Errorf("file changed on a refused edit:\n%s", got)
			}
		})
	}
}

// A hand-broken file is the user's to fix; writing over it would destroy what
// they were in the middle of.
func TestSetOverride_RefusesInvalidExistingFile(t *testing.T) {
	path := overridePathIn(t)
	const orig = "language: [ko\n"
	writeOverride(t, path, orig)

	if err := SetOverride(path, "experimental", true); err == nil || !strings.Contains(err.Error(), "tacit config edit") {
		t.Fatalf("err = %v, want one pointing at 'tacit config edit'", err)
	}
	if got := readOverride(t, path); got != orig {
		t.Errorf("file changed: %q", got)
	}
}

// A hand-written bare 0 loads, so it must not make the file refuse other edits.
func TestSetOverride_AcceptsExistingBareZeroDuration(t *testing.T) {
	path := overridePathIn(t)
	writeOverride(t, path, "dedup_window: 0\n")

	if err := SetOverride(path, "language", "ko"); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}
	if got, want := readOverride(t, path), "dedup_window: 0\nlanguage: ko\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if cfg := loadOverride(t, path); cfg.DedupWindow != 0 || cfg.Language != "ko" {
		t.Errorf("DedupWindow = %v, Language = %q; want 0, ko", cfg.DedupWindow, cfg.Language)
	}
}

func TestClearOverride_CommentsOutWithDefault(t *testing.T) {
	path := overridePathIn(t)
	writeOverride(t, path, "language: ko\nsilence_duration: 8s\ntranscript_denylist:\n  - a\n")

	if err := ClearOverride(path, "silence_duration"); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}
	if err := ClearOverride(path, "transcript_denylist"); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}

	want := "language: ko\n# silence_duration: 10s\n# transcript_denylist: []\n"
	if got := readOverride(t, path); got != want {
		t.Errorf("file =\n%s\nwant =\n%s", got, want)
	}
	keys, err := LoadOverrideKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if keys["silence_duration"] || keys["transcript_denylist"] || !keys["language"] {
		t.Errorf("override keys after clear = %v", keys)
	}
}

func TestClearOverride_NoOpWhenNotSet(t *testing.T) {
	path := overridePathIn(t)
	const orig = "language: ko   # keep\n# silence_duration: 3s\n"
	writeOverride(t, path, orig)
	info, _ := os.Stat(path)

	if err := ClearOverride(path, "silence_duration"); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}
	if got := readOverride(t, path); got != orig {
		t.Errorf("file changed: %q", got)
	}
	if after, _ := os.Stat(path); !after.ModTime().Equal(info.ModTime()) {
		t.Error("file was rewritten for a no-op clear")
	}

	missing := overridePathIn(t)
	if err := ClearOverride(missing, "language"); err != nil {
		t.Fatalf("ClearOverride on missing file: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("clearing on a missing file created it")
	}
}

func TestSetThenClear_RestoresDefault(t *testing.T) {
	path := overridePathIn(t)
	if err := SetOverride(path, "min_char_rate", 0.5); err != nil {
		t.Fatal(err)
	}
	if err := ClearOverride(path, "min_char_rate"); err != nil {
		t.Fatal(err)
	}
	if got := readOverride(t, path); got != overrideTemplate(DefaultConfig()) {
		t.Errorf("set+clear did not restore the template:\n%s", got)
	}
}

func TestKeys_CoversEveryField(t *testing.T) {
	keys := Keys()
	if n := reflect.TypeOf(Config{}).NumField(); len(keys) != n {
		t.Fatalf("Keys() has %d keys, Config has %d fields", len(keys), n)
	}
	if keys[0] != "whisper_model" {
		t.Errorf("Keys()[0] = %q, want declaration order starting at whisper_model", keys[0])
	}
	// Every key must be one the template knows, or SetOverride on a fresh
	// file would append it instead of uncommenting it.
	tmpl := overrideTemplate(DefaultConfig())
	for _, k := range keys {
		if !strings.Contains(tmpl, "\n# "+k+":") {
			t.Errorf("template has no line for %q", k)
		}
	}
}

func TestFields(t *testing.T) {
	path := overridePathIn(t)
	writeOverride(t, path, "language: ko\nsilence_duration: 8s\n")

	fields, err := Fields("", path)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	if len(fields) != len(Keys()) {
		t.Fatalf("got %d fields, want %d", len(fields), len(Keys()))
	}
	byKey := map[string]Field{}
	for i, f := range fields {
		if f.Key != Keys()[i] {
			t.Errorf("fields[%d] = %q, want %q (declaration order)", i, f.Key, Keys()[i])
		}
		byKey[f.Key] = f
	}

	if f := byKey["language"]; f.Value != "ko" || f.Default != "auto" || !f.Overridden {
		t.Errorf("language = %+v", f)
	}
	if f := byKey["silence_duration"]; f.Value != "8s" || f.Default != "10s" || !f.Overridden {
		t.Errorf("silence_duration = %+v, want durations in file form", f)
	}
	if f := byKey["experimental"]; f.Value != false || f.Overridden {
		t.Errorf("experimental = %+v", f)
	}
	if f := byKey["transcript_denylist"]; f.Value == nil || reflect.ValueOf(f.Value).Len() != 0 {
		t.Errorf("transcript_denylist = %#v, want an empty non-nil list", f.Value)
	}
	for key, want := range map[string]string{
		"language": "string", "experimental": "bool", "speech_threshold": "number",
		"silence_duration": "duration", "transcript_denylist": "list",
	} {
		if got := byKey[key].Kind; got != want {
			t.Errorf("%s kind = %q, want %q", key, got, want)
		}
	}

	// A value read here has to be writable straight back.
	for _, f := range fields {
		if err := SetOverride(path, f.Key, f.Value); err != nil {
			t.Errorf("SetOverride(%s, Fields value %#v): %v", f.Key, f.Value, err)
		}
	}
}

func TestParseValue(t *testing.T) {
	cases := []struct {
		key, text string
		want      any
	}{
		{"language", "ko", "ko"},
		{"initial_prompt", "tacit: whisper, VAD", "tacit: whisper, VAD"},
		{"initial_prompt", "", ""},
		{"llm_model", "3.5", "3.5"},
		{"experimental", "true", true},
		{"speech_threshold", "0.6", 0.6},
		{"energy_threshold", "300", 300},
		{"silence_duration", "8s", "8s"},
		{"transcript_denylist", "[a, 구독과 좋아요]", []any{"a", "구독과 좋아요"}},
		{"transcript_denylist", "구독과 좋아요", []any{"구독과 좋아요"}},
		{"transcript_denylist", "[]", []any{}},
		{"transcript_denylist", "", []any{}},
	}
	for _, c := range cases {
		got, err := ParseValue(c.key, c.text)
		if err != nil {
			t.Errorf("ParseValue(%s, %q): %v", c.key, c.text, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseValue(%s, %q) = %#v, want %#v", c.key, c.text, got, c.want)
		}
	}
	if _, err := ParseValue("nope", "x"); err == nil {
		t.Error("ParseValue accepted an unknown key")
	}
}

// Whatever ParseValue produces for typical command-line input must be
// accepted by SetOverride and load back as the intended setting.
func TestParseValue_FeedsSetOverride(t *testing.T) {
	path := overridePathIn(t)
	for key, text := range map[string]string{
		"initial_prompt":      "tacit: whisper, VAD",
		"experimental":        "true",
		"min_char_rate":       "0.35",
		"dedup_window":        "0s",
		"transcript_denylist": "구독과 좋아요",
	} {
		v, err := ParseValue(key, text)
		if err != nil {
			t.Fatalf("ParseValue(%s): %v", key, err)
		}
		if err := SetOverride(path, key, v); err != nil {
			t.Fatalf("SetOverride(%s, %#v): %v", key, v, err)
		}
	}
	cfg := loadOverride(t, path)
	if cfg.InitialPrompt != "tacit: whisper, VAD" || !cfg.Experimental || cfg.MinCharRate != 0.35 ||
		cfg.DedupWindow != 0 || !reflect.DeepEqual(cfg.TranscriptDenylist, []string{"구독과 좋아요"}) {
		t.Errorf("loaded config = %+v", cfg)
	}
}
