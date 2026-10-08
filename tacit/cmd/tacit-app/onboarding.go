package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/model"
	"github.com/sangmin7648/tacit/pkg/setup"
)

// modelProgressEvent carries model download progress to the onboarding
// window. frontend/src/backend.js subscribes to it by this exact name.
const modelProgressEvent = "onboarding:model-progress"

// OnboardingService is what the onboarding window calls. Every decision it
// makes is pkg/setup's — the same code `tacit setup` runs — so the window and
// the terminal wizard cannot drift apart.
type OnboardingService struct {
	tray *trayApp

	mu     sync.Mutex
	window *application.WebviewWindow
}

// OnboardingOptions is everything the window needs to draw its form.
type OnboardingOptions struct {
	// Configured is false on a first run: nothing has been set up yet.
	Configured         bool             `json:"configured"`
	Choices            setup.Choices    `json:"choices"`
	Providers          []string         `json:"providers"`
	ClaudeModels       []string         `json:"claude_models"`
	Languages          []setup.Language `json:"languages"`
	DefaultOllamaModel string           `json:"default_ollama_model"`
	Model              ModelInfo        `json:"model"`
}

// ModelInfo describes the whisper model setup will need.
type ModelInfo struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
}

// Permissions is the app's (and so the daemon's) capture permission state.
type Permissions struct {
	Microphone string `json:"microphone"`
}

// ModelProgress is the payload of modelProgressEvent.
type ModelProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// isConfigured reports whether setup has run — CLI or app. Both write the
// reference config.yaml.
func isConfigured() bool {
	_, err := os.Stat(config.ConfigPath())
	return err == nil
}

func loadConfig() (*config.Config, error) {
	return config.LoadWithOverride(config.ConfigPath(), config.OverridePath())
}

// Options returns the form's choices, pre-filled from the current config once
// setup has run, and from the defaults before that.
func (s *OnboardingService) Options() (OnboardingOptions, error) {
	cfg, err := loadConfig()
	if err != nil {
		return OnboardingOptions{}, err
	}
	configured := isConfigured()
	choices := setup.Defaults()
	if configured {
		choices = setup.FromConfig(cfg)
	}
	_, statErr := os.Stat(config.ModelPath(cfg.WhisperModel))
	return OnboardingOptions{
		Configured:         configured,
		Choices:            choices,
		Providers:          setup.Providers,
		ClaudeModels:       setup.ClaudeModels,
		Languages:          setup.Languages,
		DefaultOllamaModel: setup.DefaultOllamaModel,
		Model:              ModelInfo{Name: cfg.WhisperModel, Present: statErr == nil},
	}, nil
}

// CheckProvider checks the chosen classifier is reachable before saving.
func (s *OnboardingService) CheckProvider(ctx context.Context, c setup.Choices) error {
	return setup.CheckProvider(ctx, c)
}

// Apply saves the choices, exactly as `tacit setup` would.
func (s *OnboardingService) Apply(c setup.Choices) (*setup.Result, error) {
	return setup.Apply(c)
}

// DownloadModel fetches the configured whisper model if it is missing,
// reporting progress through modelProgressEvent. The window cancels it by
// cancelling the call, which cancels ctx; the partial file is removed.
func (s *OnboardingService) DownloadModel(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	app := application.Get()
	var last time.Time
	progress := func(done, total int64) {
		// The callback fires per network read; ten redraws a second is plenty.
		if now := time.Now(); now.Sub(last) >= 100*time.Millisecond || done == total {
			last = now
			app.Event.Emit(modelProgressEvent, ModelProgress{Done: done, Total: total})
		}
	}
	if err := model.Download(ctx, config.ModelPath(cfg.WhisperModel), progress); err != nil {
		if errors.Is(err, context.Canceled) {
			return errors.New("download cancelled")
		}
		return err
	}
	return nil
}

// Permissions reports capture permissions without ever prompting — the window
// polls it every second.
func (s *OnboardingService) Permissions() Permissions {
	return Permissions{Microphone: microphoneStatus()}
}

// RequestMicrophone shows the system microphone prompt, if it has not been
// answered. The window polls Permissions for the outcome.
func (s *OnboardingService) RequestMicrophone() { requestMicrophone() }

// OpenPrivacySettings opens a Privacy & Security pane: "Microphone".
func (s *OnboardingService) OpenPrivacySettings(pane string) error {
	switch pane {
	case "Microphone":
	default:
		return fmt.Errorf("unknown privacy pane %q", pane)
	}
	return exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_"+pane).Start()
}

// Finish closes the window, marks onboarding done, and, if asked, starts
// listening.
func (s *OnboardingService) Finish(startListening bool) {
	s.mu.Lock()
	w := s.window
	s.mu.Unlock()
	if w != nil {
		w.Hide()
	}
	if startListening {
		if running, _ := daemonStatus(config.PIDPath()); !running {
			s.tray.start()
		}
	}
}

// show opens the onboarding window, creating it the first time. Closing it
// hides it instead, so a download in progress keeps going and is still shown
// when the window is reopened.
func (s *OnboardingService) show() {
	application.InvokeSync(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.window == nil {
			w := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
				Name:   "onboarding",
				Title:  "Set Up Tacit",
				Width:  560,
				Height: 720,
				URL:    "/",
			})
			w.RegisterHook(wailsevents.Common.WindowClosing, func(e *application.WindowEvent) {
				e.Cancel()
				w.Hide()
			})
			s.window = w
		}
		application.Get().Show()
		s.window.Show()
		s.window.Focus()
	})
}
