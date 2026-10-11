// Package onboard applies the choices made during first-run setup. It holds the
// decisions `tacit setup` makes, without the prompting, so the terminal wizard
// and the Mac app's onboarding window share one implementation: each collects
// Choices its own way, then calls Apply.
package onboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	modeldownloader "github.com/sangmin7648/tacit/core/internal/components/model-downloader"
	noteclassifier "github.com/sangmin7648/tacit/core/internal/components/note-classifier"
	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
	skillinstaller "github.com/sangmin7648/tacit/core/internal/components/skill-installer"
)

// The options a front end offers. The first entry of each is the one the
// wizard pre-selects.
var (
	Providers    = []string{"ollama", "claude"}
	ClaudeModels = []string{"haiku", "sonnet", "opus"}
	Agents       = skillinstaller.AgentNames()
	Languages    = []Language{
		{Code: "auto", Label: "auto (detect)"},
		{Code: "en", Label: "english"},
		{Code: "ko", Label: "korean"},
	}
)

// DefaultOllamaModel is the model recommended when the provider is ollama,
// chosen by trying several against real transcripts. Any locally pulled model
// works, so the window offers the ones installed beside it.
var DefaultOllamaModel = settingmanager.DefaultConfig().LLMModel

// CurrentRevision numbers the onboarding a user has to go through. Bump it
// when a change leaves existing users better off re-choosing — new options,
// new recommendations, a changed default — and they are shown the window once
// more. Leave it alone for everything else: it is not the app's version, and a
// window on every update would be noise.
const CurrentRevision = 1

// Needed reports whether the user should be shown onboarding: setup has never
// run, or it ran before the current revision.
func Needed() bool {
	return !Configured() || settingmanager.OnboardedRevision() < CurrentRevision
}

// MarkSeen records the current revision as shown, without a setup having run.
// The app calls it when it opens the window for a user who is already set up,
// so closing the window instead of finishing it does not bring it back at every
// launch. A user who has never been set up is not marked: they are asked until
// they finish.
func MarkSeen() error { return settingmanager.MarkOnboarded(CurrentRevision) }

// Language is a transcription language a front end offers.
type Language struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// Choices are the answers setup collects.
type Choices struct {
	LLMProvider  string `json:"llm_provider"`
	LLMModel     string `json:"llm_model"`
	SkillAgent   string `json:"skill_agent"`
	Language     string `json:"language"`
	WhisperModel string `json:"whisper_model"`
}

// Defaults returns the answers a user gets by accepting every suggestion.
func Defaults() Choices {
	d := settingmanager.DefaultConfig()
	return Choices{
		LLMProvider:  d.LLMProvider,
		LLMModel:     d.LLMModel,
		SkillAgent:   d.SkillAgent,
		Language:     d.Language,
		WhisperModel: d.WhisperModel,
	}
}

// FromConfig returns the answers that would reproduce cfg's current settings,
// so re-running setup starts from where the user is rather than from the
// defaults.
func FromConfig(cfg *settingmanager.Config) Choices {
	return Choices{
		LLMProvider:  cfg.LLMProvider,
		LLMModel:     cfg.LLMModel,
		SkillAgent:   cfg.SkillAgent,
		Language:     cfg.Language,
		WhisperModel: cfg.WhisperModel,
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
		if !noteclassifier.ClaudeAvailable() {
			return errors.New("Claude Code CLI not found on PATH\n  → Install it: https://docs.anthropic.com/en/docs/claude-code")
		}
		return nil
	}
	cfg := settingmanager.DefaultConfig()
	cfg.LLMProvider, cfg.LLMModel = c.LLMProvider, c.LLMModel
	if p, ok := noteclassifier.NewClassifier(cfg).(noteclassifier.Pinger); ok {
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
	case strings.TrimSpace(c.Language) == "":
		return errors.New("a transcription language is required")
	case strings.TrimSpace(c.WhisperModel) == "":
		return errors.New("a speech model is required")
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
// chosen agent, regenerates the reference config.yaml and marks the current
// onboarding revision done. It stops at the
// first failure; the returned Result lists what was written up to that point.
func Apply(c Choices) (*Result, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	res := &Result{
		OverridePath:    settingmanager.OverridePath(),
		ReferencePath:   settingmanager.ConfigPath(),
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

	if err := settingmanager.WriteSetupOverride(res.OverridePath, c.LLMProvider, c.LLMModel, c.SkillAgent,
		c.Language, c.WhisperModel); err != nil {
		return res, fmt.Errorf("writing config override: %w", err)
	}

	installed, err := skillinstaller.Install(c.SkillAgent)
	res.InstalledSkills = append(res.InstalledSkills, installed...)
	if err != nil {
		return res, fmt.Errorf("installing skills: %w", err)
	}

	if err := settingmanager.WriteDefault(res.ReferencePath); err != nil {
		return res, fmt.Errorf("writing reference config: %w", err)
	}
	if err := MarkSeen(); err != nil {
		return res, fmt.Errorf("recording onboarding: %w", err)
	}
	return res, nil
}

// Configured reports whether setup has run, from the terminal or the app: both
// write the reference settings file.
func Configured() bool {
	_, err := os.Stat(settingmanager.ConfigPath())
	return err == nil
}

// ModelPath is where the whisper model cfg names is kept.
func ModelPath(cfg *settingmanager.Config) string { return settingmanager.ModelPath(cfg.WhisperModel) }

// Progress reports a model download's progress.
type Progress = modeldownloader.Progress

// PrintProgress is the terminal's Progress for downloading file: one line,
// redrawn in place.
func PrintProgress(file string) Progress { return modeldownloader.PrintProgress(file) }

// InstallSkills installs tacit's skills for agent and returns where they went.
func InstallSkills(agent string) ([]string, error) { return skillinstaller.Install(agent) }

// DownloadModel fetches the whisper model the saved settings name, unless it
// is already there. Setup ends with it, in the terminal and in the app alike,
// so the first listen does not stall on a download of a gigabyte or more.
func DownloadModel(ctx context.Context, progress modeldownloader.Progress) error {
	cfg, err := settingmanager.LoadWithOverride(settingmanager.ConfigPath(), settingmanager.OverridePath())
	if err != nil {
		return err
	}
	return modeldownloader.Download(ctx, settingmanager.ModelPath(cfg.WhisperModel), progress)
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
