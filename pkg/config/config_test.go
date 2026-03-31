package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.WhisperModel != "medium" {
		t.Errorf("WhisperModel: got %q, want %q", cfg.WhisperModel, "medium")
	}
	if cfg.MinSpeechDur != 8*time.Second {
		t.Errorf("MinSpeechDur: got %v, want %v", cfg.MinSpeechDur, 8*time.Second)
	}
	if cfg.SilenceDuration != 1500*time.Millisecond {
		t.Errorf("SilenceDuration: got %v, want %v", cfg.SilenceDuration, 1500*time.Millisecond)
	}
	if cfg.SpeechThreshold != 0.5 {
		t.Errorf("SpeechThreshold: got %v, want %v", cfg.SpeechThreshold, 0.5)
	}
	if cfg.LLMProvider != "ollama" {
		t.Errorf("LLMProvider: got %q, want %q", cfg.LLMProvider, "ollama")
	}
	if cfg.LLMModel != "qwen3.5" {
		t.Errorf("LLMModel: got %q, want %q", cfg.LLMModel, "qwen3.5")
	}
}

func TestLoadWithOverride_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `whisper_model: large
min_speech_duration: 5s
silence_duration: 2s
speech_threshold: 0.8
llm_provider: claude
llm_model: sonnet
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadWithOverride(cfgPath, "")
	if err != nil {
		t.Fatalf("LoadWithOverride returned error: %v", err)
	}

	if cfg.WhisperModel != "large" {
		t.Errorf("WhisperModel: got %q, want %q", cfg.WhisperModel, "large")
	}
	if cfg.MinSpeechDur != 5*time.Second {
		t.Errorf("MinSpeechDur: got %v, want %v", cfg.MinSpeechDur, 5*time.Second)
	}
	if cfg.SilenceDuration != 2*time.Second {
		t.Errorf("SilenceDuration: got %v, want %v", cfg.SilenceDuration, 2*time.Second)
	}
	if cfg.SpeechThreshold != 0.8 {
		t.Errorf("SpeechThreshold: got %v, want %v", cfg.SpeechThreshold, 0.8)
	}
	if cfg.LLMProvider != "claude" {
		t.Errorf("LLMProvider: got %q, want %q", cfg.LLMProvider, "claude")
	}
	if cfg.LLMModel != "sonnet" {
		t.Errorf("LLMModel: got %q, want %q", cfg.LLMModel, "sonnet")
	}
}

func TestLoadWithOverride_NoFiles(t *testing.T) {
	cfg, err := LoadWithOverride("/nonexistent/config.yaml", "/nonexistent/config-override.yaml")
	if err != nil {
		t.Fatalf("LoadWithOverride returned error for missing files: %v", err)
	}

	defaults := DefaultConfig()
	if cfg.WhisperModel != defaults.WhisperModel {
		t.Errorf("WhisperModel: got %q, want default %q", cfg.WhisperModel, defaults.WhisperModel)
	}
	if cfg.MinSpeechDur != defaults.MinSpeechDur {
		t.Errorf("MinSpeechDur: got %v, want default %v", cfg.MinSpeechDur, defaults.MinSpeechDur)
	}
	if cfg.LLMModel != defaults.LLMModel {
		t.Errorf("LLMModel: got %q, want default %q", cfg.LLMModel, defaults.LLMModel)
	}
}

func TestLoadWithOverride_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Only override two fields; the rest should keep defaults.
	content := `whisper_model: small
speech_threshold: 0.7
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadWithOverride(cfgPath, "")
	if err != nil {
		t.Fatalf("LoadWithOverride returned error: %v", err)
	}

	if cfg.WhisperModel != "small" {
		t.Errorf("WhisperModel: got %q, want %q", cfg.WhisperModel, "small")
	}
	if cfg.SpeechThreshold != 0.7 {
		t.Errorf("SpeechThreshold: got %v, want %v", cfg.SpeechThreshold, 0.7)
	}

	defaults := DefaultConfig()
	if cfg.MinSpeechDur != defaults.MinSpeechDur {
		t.Errorf("MinSpeechDur: got %v, want default %v", cfg.MinSpeechDur, defaults.MinSpeechDur)
	}
	if cfg.SilenceDuration != defaults.SilenceDuration {
		t.Errorf("SilenceDuration: got %v, want default %v", cfg.SilenceDuration, defaults.SilenceDuration)
	}
	if cfg.LLMModel != defaults.LLMModel {
		t.Errorf("LLMModel: got %q, want default %q", cfg.LLMModel, defaults.LLMModel)
	}
}

func TestLoadWithOverride_OverrideWins(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	overridePath := filepath.Join(dir, "config-override.yaml")

	if err := os.WriteFile(cfgPath, []byte("llm_model: base-model\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := os.WriteFile(overridePath, []byte("llm_model: override-model\n"), 0644); err != nil {
		t.Fatalf("failed to write override: %v", err)
	}

	cfg, err := LoadWithOverride(cfgPath, overridePath)
	if err != nil {
		t.Fatalf("LoadWithOverride returned error: %v", err)
	}

	if cfg.LLMModel != "override-model" {
		t.Errorf("LLMModel: got %q, want %q", cfg.LLMModel, "override-model")
	}
}

func TestLoadWithOverride_PartialOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	overridePath := filepath.Join(dir, "config-override.yaml")

	cfgContent := `whisper_model: large
llm_model: base-model
llm_provider: ollama
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	// Override only llm_model.
	if err := os.WriteFile(overridePath, []byte("llm_model: override-model\n"), 0644); err != nil {
		t.Fatalf("failed to write override: %v", err)
	}

	cfg, err := LoadWithOverride(cfgPath, overridePath)
	if err != nil {
		t.Fatalf("LoadWithOverride returned error: %v", err)
	}

	if cfg.LLMModel != "override-model" {
		t.Errorf("LLMModel: got %q, want %q", cfg.LLMModel, "override-model")
	}
	// Non-overridden fields from config.yaml should be preserved.
	if cfg.WhisperModel != "large" {
		t.Errorf("WhisperModel: got %q, want %q", cfg.WhisperModel, "large")
	}
	if cfg.LLMProvider != "ollama" {
		t.Errorf("LLMProvider: got %q, want %q", cfg.LLMProvider, "ollama")
	}
}

func TestLoadOverrideKeys_Empty(t *testing.T) {
	keys, err := LoadOverrideKeys("/nonexistent/config-override.yaml")
	if err != nil {
		t.Fatalf("LoadOverrideKeys returned error for missing file: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty keys map, got %v", keys)
	}
}

func TestLoadOverrideKeys_PartialYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config-override.yaml")

	content := `llm_model: sonnet
llm_provider: claude
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write override: %v", err)
	}

	keys, err := LoadOverrideKeys(path)
	if err != nil {
		t.Fatalf("LoadOverrideKeys returned error: %v", err)
	}

	if !keys["llm_model"] {
		t.Error("expected llm_model to be in override keys")
	}
	if !keys["llm_provider"] {
		t.Error("expected llm_provider to be in override keys")
	}
	if keys["whisper_model"] {
		t.Error("expected whisper_model NOT to be in override keys")
	}
}

func TestWriteOverrideTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config-override.yaml")

	defaults := DefaultConfig()
	if err := WriteOverrideTemplate(path, defaults); err != nil {
		t.Fatalf("WriteOverrideTemplate returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	content := string(data)

	// Should start with the header comment.
	if len(content) == 0 || content[0] != '#' {
		t.Error("expected file to start with a comment")
	}

	// All field lines should be commented out — unmarshaling should yield zero/empty Config.
	cfg := &Config{}
	if err := loadFile(path, cfg); err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}
	if cfg.WhisperModel != "" || cfg.LLMModel != "" || cfg.LLMProvider != "" {
		t.Errorf("expected all fields to be empty (commented out), got WhisperModel=%q LLMModel=%q LLMProvider=%q",
			cfg.WhisperModel, cfg.LLMModel, cfg.LLMProvider)
	}

	// Default field values should appear as comments.
	if !containsLine(content, defaults.WhisperModel) {
		t.Errorf("expected template to mention default whisper_model %q", defaults.WhisperModel)
	}
	if !containsLine(content, defaults.LLMModel) {
		t.Errorf("expected template to mention default llm_model %q", defaults.LLMModel)
	}
}

func TestBaseDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}

	want := filepath.Join(home, ".tacit")
	got := BaseDir()
	if got != want {
		t.Errorf("BaseDir: got %q, want %q", got, want)
	}
}

// containsLine reports whether s contains the given substring on any line.
func containsLine(s, substr string) bool {
	return len(substr) > 0 && len(s) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsRaw(s, substr))
}

func containsRaw(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
