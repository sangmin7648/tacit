package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds user-configurable settings for the tacit pipeline.
// Stored at ~/.tacit/config.yaml; missing file means all defaults apply.
type Config struct {
	WhisperModel    string        `yaml:"whisper_model"`
	InitialPrompt   string        `yaml:"initial_prompt"`
	MinSpeechDur    time.Duration `yaml:"min_speech_duration"`
	SilenceDuration time.Duration `yaml:"silence_duration"`
	SpeechThreshold float64       `yaml:"speech_threshold"`
	EnergyThreshold float64       `yaml:"energy_threshold"`
	LLMProvider     string        `yaml:"llm_provider"`
	LLMModel        string        `yaml:"llm_model"`
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		WhisperModel:    "medium",
		MinSpeechDur:    8 * time.Second,
		SilenceDuration: 1500 * time.Millisecond,
		SpeechThreshold: 0.5,
		EnergyThreshold: 200,
		LLMProvider:     "ollama",
		LLMModel:        "qwen3.5",
	}
}

// Load reads a YAML config file at path and returns the resulting Config.
// If the file does not exist, it returns DefaultConfig with a nil error.
// Fields absent from the YAML file retain their default values and are
// written back to the file so users can discover and edit them.
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

	// Write back if any new fields are missing from the file.
	updated, err := yaml.Marshal(cfg)
	if err == nil && string(updated) != string(data) {
		_ = os.WriteFile(path, updated, 0644)
	}

	return cfg, nil
}

// WriteDefault writes a default config YAML file to path.
// Useful for seeding a config file before opening it in an editor.
func WriteDefault(path string) error {
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// BaseDir returns the root directory for the tacit knowledge base (~/.tacit).
func BaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback: this should not happen on supported platforms.
		return filepath.Join(os.Getenv("HOME"), ".tacit")
	}
	return filepath.Join(home, ".tacit")
}

// ConfigPath returns the default config file path (~/.tacit/config.yaml).
func ConfigPath() string {
	return filepath.Join(BaseDir(), "config.yaml")
}

// ModelPath returns the path for a whisper model file (~/.tacit/models/ggml-{model}.bin).
func ModelPath(model string) string {
	return filepath.Join(BaseDir(), "models", "ggml-"+model+".bin")
}

// PIDPath returns the path for the daemon PID file (~/.tacit/tacit.pid).
func PIDPath() string {
	return filepath.Join(BaseDir(), "tacit.pid")
}
