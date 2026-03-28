package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.WhisperModel != "base" {
		t.Errorf("WhisperModel: got %q, want %q", cfg.WhisperModel, "base")
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
	if cfg.ClaudeModel != "haiku" {
		t.Errorf("ClaudeModel: got %q, want %q", cfg.ClaudeModel, "haiku")
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `whisper_model: large
min_speech_duration: 5s
silence_duration: 2s
speech_threshold: 0.8
claude_model: sonnet
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
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
	if cfg.ClaudeModel != "sonnet" {
		t.Errorf("ClaudeModel: got %q, want %q", cfg.ClaudeModel, "sonnet")
	}
}

func TestLoadFileNotExist(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}

	// Should return defaults.
	defaults := DefaultConfig()
	if cfg.WhisperModel != defaults.WhisperModel {
		t.Errorf("WhisperModel: got %q, want default %q", cfg.WhisperModel, defaults.WhisperModel)
	}
	if cfg.MinSpeechDur != defaults.MinSpeechDur {
		t.Errorf("MinSpeechDur: got %v, want default %v", cfg.MinSpeechDur, defaults.MinSpeechDur)
	}
	if cfg.SilenceDuration != defaults.SilenceDuration {
		t.Errorf("SilenceDuration: got %v, want default %v", cfg.SilenceDuration, defaults.SilenceDuration)
	}
	if cfg.SpeechThreshold != defaults.SpeechThreshold {
		t.Errorf("SpeechThreshold: got %v, want default %v", cfg.SpeechThreshold, defaults.SpeechThreshold)
	}
	if cfg.ClaudeModel != defaults.ClaudeModel {
		t.Errorf("ClaudeModel: got %q, want default %q", cfg.ClaudeModel, defaults.ClaudeModel)
	}
}

func TestLoadPartialYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Only override two fields; the rest should keep defaults.
	content := `whisper_model: small
speech_threshold: 0.7
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Overridden fields.
	if cfg.WhisperModel != "small" {
		t.Errorf("WhisperModel: got %q, want %q", cfg.WhisperModel, "small")
	}
	if cfg.SpeechThreshold != 0.7 {
		t.Errorf("SpeechThreshold: got %v, want %v", cfg.SpeechThreshold, 0.7)
	}

	// Default fields.
	defaults := DefaultConfig()
	if cfg.MinSpeechDur != defaults.MinSpeechDur {
		t.Errorf("MinSpeechDur: got %v, want default %v", cfg.MinSpeechDur, defaults.MinSpeechDur)
	}
	if cfg.SilenceDuration != defaults.SilenceDuration {
		t.Errorf("SilenceDuration: got %v, want default %v", cfg.SilenceDuration, defaults.SilenceDuration)
	}
	if cfg.ClaudeModel != defaults.ClaudeModel {
		t.Errorf("ClaudeModel: got %q, want default %q", cfg.ClaudeModel, defaults.ClaudeModel)
	}
}

func TestBaseDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}

	want := filepath.Join(home, ".sttdb")
	got := BaseDir()
	if got != want {
		t.Errorf("BaseDir: got %q, want %q", got, want)
	}
}
