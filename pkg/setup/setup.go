// Package setup applies the choices made during first-run setup. It holds the
// decisions `tacit setup` makes, without the prompting, so the terminal wizard
// and the Mac app's onboarding window share one implementation: each collects
// Choices its own way, then calls Apply.
package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/process"
	"github.com/sangmin7648/tacit/skills"
)

// The options a front end offers. The first entry of each is the one the
// wizard pre-selects.
var (
	Providers    = []string{"ollama", "claude"}
	ClaudeModels = []string{"haiku", "sonnet", "opus"}
	Agents       = []string{"claude"}
	Languages    = []Language{
		{Code: "auto", Label: "auto (detect)"},
		{Code: "en", Label: "english"},
		{Code: "ko", Label: "korean"},
	}
)

// DefaultOllamaModel is pre-filled when the provider is ollama. The model is
// free text there, since any locally pulled model works.
var DefaultOllamaModel = config.DefaultConfig().LLMModel

// Language is a transcription language a front end offers.
type Language struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// Choices are the answers setup collects.
type Choices struct {
	LLMProvider    string `json:"llm_provider"`
	LLMModel       string `json:"llm_model"`
	SkillAgent     string `json:"skill_agent"`
	CaptureMic     bool   `json:"capture_mic"`
	CaptureSpeaker bool   `json:"capture_speaker"`
	Language       string `json:"language"`
	Experimental   bool   `json:"experimental"`
}

// Defaults returns the answers a user gets by accepting every suggestion.
func Defaults() Choices {
	d := config.DefaultConfig()
	return Choices{
		LLMProvider:    d.LLMProvider,
		LLMModel:       d.LLMModel,
		SkillAgent:     d.SkillAgent,
		CaptureMic:     d.CaptureMic,
		CaptureSpeaker: d.CaptureSpeaker,
		Language:       d.Language,
		Experimental:   d.Experimental,
	}
}

// FromConfig returns the answers that would reproduce cfg's current settings,
// so re-running setup starts from where the user is rather than from the
// defaults.
func FromConfig(cfg *config.Config) Choices {
	return Choices{
		LLMProvider:    cfg.LLMProvider,
		LLMModel:       cfg.LLMModel,
		SkillAgent:     cfg.SkillAgent,
		CaptureMic:     cfg.CaptureMic,
		CaptureSpeaker: cfg.CaptureSpeaker,
		Language:       cfg.Language,
		Experimental:   cfg.Experimental,
	}
}

// CheckProvider reports whether the classifier c chose can be reached, before
// anything is saved. `tacit listen` refuses to start when it cannot, so this
// is the same check caught earlier: for ollama, the server is up and has the
// model; for claude, the Claude Code CLI is on PATH.
func CheckProvider(ctx context.Context, c Choices) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.LLMProvider == "claude" {
		if _, err := exec.LookPath("claude"); err != nil {
			return errors.New("Claude Code CLI not found on PATH\n  → Install it: https://docs.anthropic.com/en/docs/claude-code")
		}
		return nil
	}
	cfg := config.DefaultConfig()
	cfg.LLMProvider, cfg.LLMModel = c.LLMProvider, c.LLMModel
	if p, ok := process.NewClassifier(cfg).(process.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

// Validate reports the first answer setup cannot proceed with.
func (c Choices) Validate() error {
	switch {
	case !slices.Contains(Providers, c.LLMProvider):
		return fmt.Errorf("unknown LLM provider %q (want one of %s)", c.LLMProvider, strings.Join(Providers, ", "))
	case strings.TrimSpace(c.LLMModel) == "":
		return errors.New("an LLM model is required")
	case c.LLMProvider == "claude" && !slices.Contains(ClaudeModels, c.LLMModel):
		return fmt.Errorf("unknown Claude model %q (want one of %s)", c.LLMModel, strings.Join(ClaudeModels, ", "))
	case !slices.Contains(Agents, c.SkillAgent):
		return fmt.Errorf("unknown skill agent %q (want one of %s)", c.SkillAgent, strings.Join(Agents, ", "))
	case !c.CaptureMic && !c.CaptureSpeaker:
		return errors.New("at least one audio source must be selected")
	case strings.TrimSpace(c.Language) == "":
		return errors.New("a transcription language is required")
	}
	return nil
}

// Result is what Apply wrote.
type Result struct {
	OverridePath  string `json:"override_path"`
	ReferencePath string `json:"reference_path"`
	// BackupPath is where a hand-edited config.yaml from before overrides
	// existed was copied before being regenerated; empty if there was none.
	BackupPath      string   `json:"backup_path,omitempty"`
	InstalledSkills []string `json:"installed_skills"`
}

// Apply records c: it writes the override file, installs the skills for the
// chosen agent, and regenerates the reference config.yaml. It stops at the
// first failure; the returned Result lists what was written up to that point.
func Apply(c Choices) (*Result, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	res := &Result{
		OverridePath:    config.OverridePath(),
		ReferencePath:   config.ConfigPath(),
		InstalledSkills: []string{},
	}
	if err := os.MkdirAll(filepath.Dir(res.OverridePath), 0o755); err != nil {
		return res, fmt.Errorf("creating config directory: %w", err)
	}

	// Checked before the override file is written: the test is "has this user
	// never had an override file?", and writing it first made the answer
	// always no, so a pre-override config.yaml was overwritten without the
	// backup this exists to make.
	backup, err := backupLegacyReference(res.ReferencePath, res.OverridePath)
	if err != nil {
		return res, err
	}
	res.BackupPath = backup

	if err := config.WriteSetupOverride(res.OverridePath, c.LLMProvider, c.LLMModel, c.SkillAgent,
		c.Language, c.CaptureMic, c.CaptureSpeaker, c.Experimental); err != nil {
		return res, fmt.Errorf("writing config override: %w", err)
	}

	installed, err := skills.Install(c.SkillAgent)
	res.InstalledSkills = append(res.InstalledSkills, installed...)
	if err != nil {
		return res, fmt.Errorf("installing skills: %w", err)
	}

	if err := config.WriteDefault(res.ReferencePath); err != nil {
		return res, fmt.Errorf("writing reference config: %w", err)
	}
	return res, nil
}

// backupLegacyReference copies config.yaml aside when it looks hand-edited and
// no override file exists yet — a user from before config-override.yaml, whose
// settings lived in the file setup is about to regenerate. It returns the
// backup path, or "" when no backup was needed.
func backupLegacyReference(referencePath, overridePath string) (string, error) {
	if _, err := os.Stat(overridePath); err == nil {
		return "", nil
	}
	existing, err := os.ReadFile(referencePath)
	if err != nil || len(existing) == 0 || existing[0] == '#' {
		// Missing, empty, or starting with the generated header.
		return "", nil
	}
	bak := referencePath + ".bak"
	if err := os.WriteFile(bak, existing, 0o644); err != nil {
		return "", fmt.Errorf("backing up %s: %w", referencePath, err)
	}
	return bak, nil
}
