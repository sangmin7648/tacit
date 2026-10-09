package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/daemon"
	"github.com/sangmin7648/tacit/pkg/setup"
)

// SettingsService is what the settings window calls. Reading and writing are
// pkg/config's Fields, SetOverride and ClearOverride — the same code behind
// `tacit config view|set|unset` — so an edit here keeps the rest of
// config-override.yaml, comments included, and a value the file could not load
// is refused with nothing written.
type SettingsService struct {
	tray *trayApp

	mu     sync.Mutex
	window *application.WebviewWindow
}

// Settings is everything the window shows.
type Settings struct {
	Fields []config.Field `json:"fields"`
	// Path is the override file the window edits.
	Path string `json:"path"`
	// Running says a daemon is up; it read its config at start, so it needs a
	// restart to pick up a change.
	Running bool `json:"running"`
}

func (s *SettingsService) Load() (Settings, error) {
	fields, err := config.Fields(config.ConfigPath(), config.OverridePath())
	if err != nil {
		return Settings{}, err
	}
	running, _ := daemon.Status(config.PIDPath())
	return Settings{Fields: fields, Path: config.OverridePath(), Running: running}, nil
}

// Set overrides one setting and returns the settings as they now are.
func (s *SettingsService) Set(key string, value any) (Settings, error) {
	if err := checkChoices(key, value); err != nil {
		return Settings{}, err
	}
	if err := config.SetOverride(config.OverridePath(), key, value); err != nil {
		return Settings{}, err
	}
	return s.Load()
}

// Clear removes one override, so the default applies again.
func (s *SettingsService) Clear(key string) (Settings, error) {
	cur, err := s.Load()
	if err != nil {
		return Settings{}, err
	}
	for _, f := range cur.Fields {
		if f.Key == key {
			if err := checkChoices(key, f.Default); err != nil {
				return Settings{}, err
			}
		}
	}
	if err := config.ClearOverride(config.OverridePath(), key); err != nil {
		return Settings{}, err
	}
	return s.Load()
}

// checkChoices refuses a change that would leave the choices setup makes
// invalid — both audio sources off, say — which the file would load fine with
// but `tacit listen` would refuse to start on. Keys setup does not cover pass.
func checkChoices(key string, value any) error {
	cfg, err := config.LoadWithOverride(config.ConfigPath(), config.OverridePath())
	if err != nil {
		return err
	}
	// Choices' JSON names are the config keys, so the change can be applied
	// through a map.
	m := map[string]any{}
	b, _ := json.Marshal(setup.FromConfig(cfg))
	json.Unmarshal(b, &m)
	if _, ok := m[key]; !ok {
		return nil
	}
	m[key] = value
	b, _ = json.Marshal(m)
	var c setup.Choices
	if err := json.Unmarshal(b, &c); err != nil {
		return nil // a value of the wrong type; SetOverride says so
	}
	return c.Validate()
}

// Restart stops the running daemon and starts it again from the app, so it
// reads the changed settings.
func (s *SettingsService) Restart(ctx context.Context) error {
	if err := daemon.StopAndWait(ctx, config.PIDPath(), restartTimeout); err != nil {
		return fmt.Errorf("%w; start it from the menu once it has", err)
	}
	return s.tray.start()
}

// EditFile opens the override file in the default text editor, creating it
// from the commented template first, as `tacit config edit` does. It is the
// way out when the file has a mistake the window cannot load past.
func (s *SettingsService) EditFile() error {
	path := config.OverridePath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := config.WriteOverrideTemplate(path, config.DefaultConfig()); err != nil {
			return err
		}
	}
	return exec.Command("open", "-t", path).Start()
}

// SetUp opens the onboarding window, where the classifier is changed: it
// checks the new one can be reached before saving it.
func (s *SettingsService) SetUp() {
	go s.tray.onboarding.show()
}

// show opens the settings window, creating it the first time; closing hides it.
func (s *SettingsService) show() {
	application.InvokeSync(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.window == nil {
			w := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
				Name:      "settings",
				Title:     "Tacit Settings",
				Width:     640,
				Height:    720,
				MinWidth:  520,
				MinHeight: 400,
				URL:       "/?view=settings",
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
