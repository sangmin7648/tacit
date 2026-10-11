package settingmanager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds user-configurable settings for the tacit pipeline.
// Loaded by merging ~/.tacit/config.yaml (setup-managed defaults) and
// ~/.tacit/config-override.yaml (user overrides). Missing files mean
// all Go defaults apply.
type Config struct {
	WhisperModel string `yaml:"whisper_model"`
	// Language is the whisper transcription language code: "auto" (detect),
	// "en", "ko", etc. Fixing this to the spoken language (instead of "auto")
	// significantly reduces hallucinated / wrong-language transcriptions.
	Language        string        `yaml:"language"`
	InitialPrompt   string        `yaml:"initial_prompt"`
	MinSpeechDur    time.Duration `yaml:"min_speech_duration"`
	SilenceDuration time.Duration `yaml:"silence_duration"`
	SpeechThreshold float64       `yaml:"speech_threshold"`
	EnergyThreshold float64       `yaml:"energy_threshold"`
	LLMProvider     string        `yaml:"llm_provider"`
	LLMModel        string        `yaml:"llm_model"`
	SkillAgent      string        `yaml:"skill_agent"`
	// MaxSegmentDur caps the maximum length of a single speech segment sent to
	// STT. When a segment grows beyond this, it is force-split and transcribed
	// immediately even if speech is still ongoing. This prevents unbounded
	// memory growth when capturing continuous audio (e.g. long videos).
	// 0 disables the cap. Default: 30s.
	MaxSegmentDur time.Duration `yaml:"max_segment_duration"`
	// MaxSessionDur caps how long transcribed text accumulates before it is sent
	// for classification. Continuous speech never triggers the silence-based
	// flush, so without this a long meeting becomes one giant item and a single
	// classification failure loses all of it. 0 disables the cap. Default: 5m.
	MaxSessionDur time.Duration `yaml:"max_session_duration"`
	// TranscriptDenylist adds phrases to the built-in list of stock sentences
	// whisper hallucinates over silence (video outros and the like). A sentence
	// is dropped when a listed phrase makes up most of it. Matching ignores
	// case, spacing and punctuation.
	TranscriptDenylist []string `yaml:"transcript_denylist"`
	// DedupWindow drops a transcript whose normalised text has already been
	// stored several times within this rolling window — the fingerprint of a
	// whisper stock hallucination, which recurs verbatim far more than real
	// speech does. The first two occurrences in any window are always kept, so
	// a genuinely repeated remark survives. 0 disables. Default 3h.
	DedupWindow time.Duration `yaml:"dedup_window"`
	// MinCharRate drops a live transcript carrying too few characters for the
	// length of audio it came from (letters and digits per second) — what is
	// left when whisper transcribes a stock phrase over a stretch it otherwise
	// read as silence. Deliberately low so only unambiguous cases are caught.
	// 0 disables. Default 0.2.
	MinCharRate float64 `yaml:"min_char_rate"`
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		WhisperModel:    "large-v3-turbo",
		Language:        "auto",
		MinSpeechDur:    2 * time.Second,
		SilenceDuration: 10 * time.Second,
		SpeechThreshold: 0.5,
		EnergyThreshold: 200,
		LLMProvider:     "ollama",
		LLMModel:        "qwen3.5",
		SkillAgent:      "claude",
		MaxSegmentDur:   30 * time.Second,
		MaxSessionDur:   5 * time.Minute,
		DedupWindow:     3 * time.Hour,
		MinCharRate:     0.2,
	}
}

// loadFile reads a YAML file at path and unmarshals it into cfg.
// If the file does not exist, it returns nil without modifying cfg.
func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return decodeConfig(data, cfg)
}

// decodeConfig decodes a config file's contents into cfg. It is the one way a
// config file is read, so what loads and what the write API accepts agree.
func decodeConfig(data []byte, cfg *Config) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		return nil // empty or comment-only file
	}
	if err := normalizeBareDurations(doc.Content[0]); err != nil {
		return err
	}
	return doc.Content[0].Decode(cfg)
}

// normalizeBareDurations applies normalizeDuration to every duration written
// as a bare number. The docs say 0 disables several durations, but YAML cannot
// decode a bare 0 into a time.Duration; any other number is refused with the
// unit it was probably meant to have, rather than read as nanoseconds.
func normalizeBareDurations(root *yaml.Node) error {
	if root.Kind != yaml.MappingNode {
		return nil // let Decode report the type error
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, val := root.Content[i], root.Content[i+1]
		f, ok := fieldByKey(key.Value)
		if !ok || f.Type != durationType || val.Kind != yaml.ScalarNode ||
			(val.Tag != "!!int" && val.Tag != "!!float") {
			continue
		}
		var v any
		if err := val.Decode(&v); err != nil {
			return err
		}
		norm, err := normalizeDuration(f, v)
		if err != nil {
			return fmt.Errorf("line %d: %w", val.Line, err)
		}
		if s, ok := norm.(string); ok {
			val.Tag, val.Value = "!!str", s
		}
	}
	return nil
}

// LoadWithOverride merges configuration from two YAML files into a single Config.
// Load order: DefaultConfig() → configPath → overridePath.
// Either path may be empty or nonexistent; missing files are silently skipped.
func LoadWithOverride(configPath, overridePath string) (*Config, error) {
	cfg := DefaultConfig()
	if configPath != "" {
		if err := loadFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("loading %s: %w", configPath, err)
		}
	}
	if overridePath != "" {
		if err := loadFile(overridePath, cfg); err != nil {
			return nil, fmt.Errorf("loading %s: %w", overridePath, err)
		}
	}
	return cfg, nil
}

// LoadOverrideKeys returns the set of YAML keys explicitly present in the
// override file at overridePath. If the file does not exist, an empty map is
// returned without error.
func LoadOverrideKeys(overridePath string) (map[string]bool, error) {
	data, err := os.ReadFile(overridePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	raw := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys, nil
}

// WriteDefault writes a reference config.yaml to path with a header comment
// that explains it is setup-managed and should not be edited by users.
// Duration fields are formatted as human-readable strings (e.g. "8s", "1.5s").
func WriteDefault(path string) error {
	cfg := DefaultConfig()
	content := fmt.Sprintf(
		"# tacit reference config — DO NOT EDIT.\n"+
			"# This file is regenerated by 'tacit setup' and documents available fields.\n"+
			"# To override values, edit config-override.yaml in the same directory.\n\n"+
			"whisper_model: %s\n"+
			"language: %s\n"+
			"initial_prompt: \"\"\n"+
			"min_speech_duration: %s\n"+
			"silence_duration: %s\n"+
			"speech_threshold: %.2f\n"+
			"energy_threshold: %.0f\n"+
			"llm_provider: %s\n"+
			"llm_model: %s\n"+
			"skill_agent: %s\n"+
			"max_segment_duration: %s\n"+
			"max_session_duration: %s\n"+
			"transcript_denylist: []\n"+
			"dedup_window: %s\n"+
			"min_char_rate: %.2f\n",
		cfg.WhisperModel,
		cfg.Language,
		formatDuration(cfg.MinSpeechDur),
		formatDuration(cfg.SilenceDuration),
		cfg.SpeechThreshold,
		cfg.EnergyThreshold,
		cfg.LLMProvider,
		cfg.LLMModel,
		cfg.SkillAgent,
		formatDuration(cfg.MaxSegmentDur),
		formatDuration(cfg.MaxSessionDur),
		formatDuration(cfg.DedupWindow),
		cfg.MinCharRate,
	)
	return os.WriteFile(path, []byte(content), 0644)
}

// WriteOverrideTemplate creates a config-override.yaml template at path with
// all fields commented out. Users uncomment and set only the fields they want
// to override. The template values reflect the current defaults.
func WriteOverrideTemplate(path string, defaults *Config) error {
	return os.WriteFile(path, []byte(overrideTemplate(defaults)), 0644)
}

// overrideTemplate renders the commented-out override template. SetOverride
// starts from it when no override file exists yet, so a file it creates looks
// the same as one `tacit config edit` would have.
func overrideTemplate(defaults *Config) string {
	header := "# tacit user overrides — edit this file to customize tacit.\n" +
		"# Only fields you uncomment and set here will override the defaults.\n" +
		"# Run 'tacit config view' to see the current merged configuration.\n\n"

	fields := []string{
		fmt.Sprintf("whisper_model: %s", defaults.WhisperModel),
		fmt.Sprintf("language: %s", defaults.Language),
		fmt.Sprintf("initial_prompt: \"\""),
		fmt.Sprintf("min_speech_duration: %s", formatDuration(defaults.MinSpeechDur)),
		fmt.Sprintf("silence_duration: %s", formatDuration(defaults.SilenceDuration)),
		fmt.Sprintf("speech_threshold: %.2f", defaults.SpeechThreshold),
		fmt.Sprintf("energy_threshold: %.0f", defaults.EnergyThreshold),
		fmt.Sprintf("llm_provider: %s", defaults.LLMProvider),
		fmt.Sprintf("llm_model: %s", defaults.LLMModel),
		fmt.Sprintf("skill_agent: %s", defaults.SkillAgent),
		fmt.Sprintf("max_segment_duration: %s", formatDuration(defaults.MaxSegmentDur)),
		fmt.Sprintf("max_session_duration: %s", formatDuration(defaults.MaxSessionDur)),
		"transcript_denylist: []",
		fmt.Sprintf("dedup_window: %s", formatDuration(defaults.DedupWindow)),
		fmt.Sprintf("min_char_rate: %.2f", defaults.MinCharRate),
	}

	var sb strings.Builder
	sb.WriteString(header)
	for _, f := range fields {
		sb.WriteString("# ")
		sb.WriteString(f)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// formatDuration formats a time.Duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	// Show as decimal seconds for sub-second or fractional values (e.g. "1.5s").
	return fmt.Sprintf("%.4gs", d.Seconds())
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

// OverridePath returns the user override config file path (~/.tacit/config-override.yaml).
func OverridePath() string {
	return filepath.Join(BaseDir(), "config-override.yaml")
}

// ModelPath returns the path for a whisper model file (~/.tacit/models/ggml-{model}.bin).
func ModelPath(model string) string {
	return filepath.Join(BaseDir(), "models", "ggml-"+model+".bin")
}

// PIDPath returns the path for the daemon PID file (~/.tacit/tacit.pid).
func PIDPath() string {
	return filepath.Join(BaseDir(), "tacit.pid")
}

// EventLogPath returns the path for the daemon event log
// (~/.tacit/events.ndjson). The daemon appends to it whoever started it, so a
// front end can follow a daemon it did not spawn.
func EventLogPath() string {
	return filepath.Join(BaseDir(), "events.ndjson")
}

// WriteSetupOverride records the setup wizard's answers in the override file
// at path, creating it from the template if it does not exist.
//
// A wizard field is set only when the answer differs from the current default
// and is cleared otherwise — whatever the file held before. A user upgrading
// from an older tacit (whose setup pinned these fields even when the default
// was accepted) therefore has the stale pin removed on re-running setup, so
// later changes to DefaultConfig() reach them.
//
// Every other line — settings setup never asks about, and the user's own
// comments — is left exactly as it was: the answers are applied one key at a
// time through SetOverride and ClearOverride.
func WriteSetupOverride(path string, provider, model, agent, language, whisperModel string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := WriteOverrideTemplate(path, DefaultConfig()); err != nil {
			return err
		}
	}

	d := DefaultConfig()
	answers := []struct {
		key          string
		chosen, dflt any
	}{
		{"language", language, d.Language},
		{"whisper_model", whisperModel, d.WhisperModel},
		{"llm_provider", provider, d.LLMProvider},
		{"llm_model", model, d.LLMModel},
		{"skill_agent", agent, d.SkillAgent},
	}
	for _, a := range answers {
		var err error
		if a.chosen == a.dflt {
			err = ClearOverride(path, a.key)
		} else {
			err = SetOverride(path, a.key, a.chosen)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
