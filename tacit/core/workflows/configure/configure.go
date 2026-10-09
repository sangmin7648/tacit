// Package configure reads and changes tacit's settings: the reference file of
// defaults and the user's override file layered over it.
package configure

import (
	"errors"
	"os"
	"path/filepath"

	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
)

type (
	Settings = settingmanager.Config
	Field    = settingmanager.Field
)

// Dir is where tacit keeps its files, notes included.
func Dir() string { return settingmanager.BaseDir() }

func ReferencePath() string { return settingmanager.ConfigPath() }
func OverridePath() string  { return settingmanager.OverridePath() }

// Load returns the effective settings: defaults, overridden by the user's file.
func Load() (*Settings, error) {
	return settingmanager.LoadWithOverride(ReferencePath(), OverridePath())
}

func Defaults() *Settings { return settingmanager.DefaultConfig() }

// Fields lists every setting with its effective value and whether the user
// overrode it.
func Fields() ([]Field, error) { return settingmanager.Fields(ReferencePath(), OverridePath()) }

// OverrideKeys reports which settings the override file sets.
func OverrideKeys() (map[string]bool, error) { return settingmanager.LoadOverrideKeys(OverridePath()) }

// ParseValue converts text typed for key into the type the setting holds.
func ParseValue(key, text string) (any, error) { return settingmanager.ParseValue(key, text) }

// Set overrides one setting.
func Set(key string, value any) error { return settingmanager.SetOverride(OverridePath(), key, value) }

// Unset clears an override so the default applies.
func Unset(key string) error { return settingmanager.ClearOverride(OverridePath(), key) }

// EnsureOverrideFile creates the override file from its commented template if
// it does not exist yet, so an editor opens something to fill in. It reports
// whether it created the file.
func EnsureOverrideFile() (created bool, err error) {
	path := OverridePath()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, settingmanager.WriteOverrideTemplate(path, Defaults())
}
