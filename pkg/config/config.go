package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds user-configurable settings for the sttdb pipeline.
// Stored at ~/.sttdb/config.yaml; missing file means all defaults apply.
type Config struct {
	WhisperModel    string        `yaml:"whisper_model"`
	MinSpeechDur    time.Duration `yaml:"min_speech_duration"`
	SilenceDuration time.Duration `yaml:"silence_duration"`
	SpeechThreshold float64       `yaml:"speech_threshold"`
	ClaudeModel     string        `yaml:"claude_model"`
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		WhisperModel:    "base",
		MinSpeechDur:    3 * time.Second,
		SilenceDuration: 1500 * time.Millisecond,
		SpeechThreshold: 0.5,
		ClaudeModel:     "haiku",
	}
}

// Load reads a YAML config file at path and returns the resulting Config.
// If the file does not exist, it returns DefaultConfig with a nil error.
// Fields absent from the YAML file retain their default values.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// BaseDir returns the root directory for the sttdb knowledge base (~/.sttdb).
func BaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback: this should not happen on supported platforms.
		return filepath.Join(os.Getenv("HOME"), ".sttdb")
	}
	return filepath.Join(home, ".sttdb")
}

// ConfigPath returns the default config file path (~/.sttdb/config.yaml).
func ConfigPath() string {
	return filepath.Join(BaseDir(), "config.yaml")
}

// ModelPath returns the path for a whisper model file (~/.sttdb/models/ggml-{model}.bin).
func ModelPath(model string) string {
	return filepath.Join(BaseDir(), "models", "ggml-"+model+".bin")
}

// PIDPath returns the path for the daemon PID file (~/.sttdb/sttdb.pid).
func PIDPath() string {
	return filepath.Join(BaseDir(), "sttdb.pid")
}
