package onboard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
)

// isolate points HOME at a temp dir, so settingmanager.BaseDir() and the skills
// directory both land there instead of in the real ~/.tacit and ~/.claude.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestDefaults_AreValid(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatalf("Defaults() fails its own validation: %v", err)
	}
	// The wizard pre-selects the first option of each list; that has to be the
	// default it would otherwise apply.
	d := Defaults()
	if Providers[0] != d.LLMProvider || Agents[0] != d.SkillAgent || Languages[0].Code != d.Language {
		t.Errorf("first options %q/%q/%q disagree with defaults %+v", Providers[0], Agents[0], Languages[0].Code, d)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]func(*Choices){
		"unknown provider":   func(c *Choices) { c.LLMProvider = "openai" },
		"empty model":        func(c *Choices) { c.LLMModel = "  " },
		"bad claude model":   func(c *Choices) { c.LLMProvider, c.LLMModel = "claude", "qwen3.5" },
		"unknown agent":      func(c *Choices) { c.SkillAgent = "cursor" },
		"empty language":     func(c *Choices) { c.Language = "" },
		"empty speech model": func(c *Choices) { c.WhisperModel = "" },
	}
	for name, mutate := range cases {
		c := Defaults()
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate accepted %+v", name, c)
		}
	}

	ok := Defaults()
	ok.LLMProvider, ok.LLMModel = "claude", "opus"
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate rejected a valid choice set: %v", err)
	}
}

func TestApply_RefusesInvalidWithoutWriting(t *testing.T) {
	home := isolate(t)
	c := Defaults()
	c.LLMProvider = "openai"

	if _, err := Apply(c); err == nil {
		t.Fatal("Apply accepted an unknown provider")
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("Apply wrote %d entries into HOME on a refused run", len(entries))
	}
}

func TestApply_WritesEverything(t *testing.T) {
	home := isolate(t)
	c := Choices{
		LLMProvider: "claude", LLMModel: "sonnet", SkillAgent: "claude",
		Language: "ko", WhisperModel: "small", Experimental: true,
	}

	res, err := Apply(c)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cfg, err := settingmanager.LoadWithOverride(res.ReferencePath, res.OverridePath)
	if err != nil {
		t.Fatalf("loading what Apply wrote: %v", err)
	}
	if cfg.LLMProvider != "claude" || cfg.LLMModel != "sonnet" || cfg.Language != "ko" ||
		cfg.WhisperModel != "small" || !cfg.Experimental {
		t.Errorf("loaded config does not reflect the choices: %+v", cfg)
	}

	ref, err := os.ReadFile(res.ReferencePath)
	if err != nil || !strings.HasPrefix(string(ref), "# tacit reference config") {
		t.Errorf("reference config not regenerated (err %v)", err)
	}

	if len(res.InstalledSkills) == 0 {
		t.Fatal("no skills reported installed")
	}
	skillsDir := filepath.Join(home, ".claude", "skills")
	for _, p := range res.InstalledSkills {
		if !strings.HasPrefix(p, skillsDir) {
			t.Errorf("skill installed outside %s: %s", skillsDir, p)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("reported skill %s missing: %v", p, err)
		}
	}
	if res.BackupPath != "" {
		t.Errorf("BackupPath = %q on a fresh install", res.BackupPath)
	}
	if Needed() {
		t.Error("onboarding still needed right after Apply")
	}
}

func TestNeeded(t *testing.T) {
	isolate(t)
	if !Needed() {
		t.Error("a first run does not need onboarding")
	}

	if _, err := Apply(Defaults()); err != nil {
		t.Fatal(err)
	}
	if Needed() {
		t.Error("needed right after setup")
	}

	// A user set up before revisions existed has settings but no marker.
	if err := os.Remove(settingmanager.OnboardedPath()); err != nil {
		t.Fatal(err)
	}
	if !Needed() {
		t.Error("a user from before revisions is not asked once")
	}
	if err := MarkSeen(); err != nil {
		t.Fatal(err)
	}
	if Needed() {
		t.Error("still needed after the window was shown")
	}

	if err := settingmanager.MarkOnboarded(CurrentRevision - 1); err != nil {
		t.Fatal(err)
	}
	if !Needed() {
		t.Error("an older revision is not asked again")
	}
}

func TestMarkSeen_DoesNotMakeAFirstRunConfigured(t *testing.T) {
	isolate(t)
	if err := MarkSeen(); err != nil {
		t.Fatal(err)
	}
	if !Needed() {
		t.Error("a user who never finished setup must be asked until they do")
	}
}

// A user from before config-override.yaml kept their settings in config.yaml,
// which setup regenerates. It has to be copied aside first.
func TestApply_BacksUpHandEditedLegacyReference(t *testing.T) {
	isolate(t)
	ref := settingmanager.ConfigPath()
	if err := os.MkdirAll(filepath.Dir(ref), 0o755); err != nil {
		t.Fatal(err)
	}
	const legacy = "whisper_model: small\nlanguage: ko\n"
	if err := os.WriteFile(ref, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(Defaults())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.BackupPath != ref+".bak" {
		t.Fatalf("BackupPath = %q, want %q", res.BackupPath, ref+".bak")
	}
	got, err := os.ReadFile(res.BackupPath)
	if err != nil || string(got) != legacy {
		t.Errorf("backup = %q (err %v), want the hand-edited file", got, err)
	}
}

// Once an override file exists the user is past the migration, and config.yaml
// is tacit's to regenerate — even if they scribbled in it.
func TestApply_NoBackupOnceOverridesExist(t *testing.T) {
	isolate(t)
	ref := settingmanager.ConfigPath()
	if err := os.MkdirAll(filepath.Dir(ref), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(ref, []byte("whisper_model: small\n"), 0o644)
	os.WriteFile(settingmanager.OverridePath(), []byte("# mine\n"), 0o644)

	res, err := Apply(Defaults())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.BackupPath != "" {
		t.Errorf("BackupPath = %q, want none", res.BackupPath)
	}
	if _, err := os.Stat(ref + ".bak"); !os.IsNotExist(err) {
		t.Error("a .bak was written")
	}
}

// Re-running setup keeps what the user wrote into the override file that setup
// never asks about.
func TestApply_RerunKeepsUserLines(t *testing.T) {
	isolate(t)
	if err := os.MkdirAll(settingmanager.BaseDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	const mine = "# pinned for the office mic\nsilence_duration: 12s\nllm_provider: claude\nllm_model: opus\n"
	if err := os.WriteFile(settingmanager.OverridePath(), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(Defaults()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, _ := os.ReadFile(settingmanager.OverridePath())
	for _, want := range []string{"# pinned for the office mic\n", "silence_duration: 12s\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("override lost %q:\n%s", want, got)
		}
	}
	cfg, err := settingmanager.LoadWithOverride("", settingmanager.OverridePath())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMProvider != "ollama" || cfg.LLMModel != settingmanager.DefaultConfig().LLMModel {
		t.Errorf("accepting defaults did not clear the provider pin: %s/%s", cfg.LLMProvider, cfg.LLMModel)
	}
}

// Setup reopened on a configured machine has to start from the user's current
// answers, and saving them unchanged has to change nothing.
func TestFromConfig_RoundTrips(t *testing.T) {
	isolate(t)
	want := Choices{
		LLMProvider: "claude", LLMModel: "opus", SkillAgent: "claude",
		Language: "ko", WhisperModel: "small", Experimental: true,
	}
	res, err := Apply(want)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	cfg, err := settingmanager.LoadWithOverride(res.ReferencePath, res.OverridePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := FromConfig(cfg); got != want {
		t.Fatalf("FromConfig = %+v, want %+v", got, want)
	}

	before, _ := os.ReadFile(res.OverridePath)
	if _, err := Apply(FromConfig(cfg)); err != nil {
		t.Fatalf("re-Apply: %v", err)
	}
	after, _ := os.ReadFile(res.OverridePath)
	if string(before) != string(after) {
		t.Errorf("re-applying the current answers changed the override file:\n%s\n---\n%s", before, after)
	}
}

// The claude check is a PATH lookup — the one that failed silently for a
// daemon started from Finder, whose PATH does not include ~/.local/bin.
func TestCheckProvider_Claude(t *testing.T) {
	c := Defaults()
	c.LLMProvider, c.LLMModel = "claude", "haiku"

	t.Setenv("PATH", t.TempDir())
	if err := CheckProvider(context.Background(), c); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want Claude CLI not found", err)
	}

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := CheckProvider(context.Background(), c); err != nil {
		t.Errorf("CheckProvider with claude on PATH: %v", err)
	}
}

func TestCheckProvider_RejectsInvalidChoices(t *testing.T) {
	c := Defaults()
	c.LLMProvider = "openai"
	if err := CheckProvider(context.Background(), c); err == nil {
		t.Error("CheckProvider accepted an unknown provider")
	}
}
