package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/core/workflows/control"

	"github.com/sangmin7648/tacit/core/workflows/configure"
)

func TestSettingsJS_MatchesService(t *testing.T) {
	checkBindings(t, "frontend/src/settings.js", "main.SettingsService.", reflect.TypeFor[*SettingsService]())
}

// settingsHome points the config at a fresh temp HOME with the given override
// file content ("" for none).
func settingsHome(t *testing.T, override string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if override != "" {
		if err := os.MkdirAll(configure.Dir(), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configure.OverridePath(), []byte(override), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func field(t *testing.T, s Settings, key string) configure.Field {
	t.Helper()
	for _, f := range s.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %q", key)
	return configure.Field{}
}

// fromJS decodes a value the way it arrives from the window: JSON into any,
// so numbers are float64 and lists []any.
func fromJS(t *testing.T, js string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSettings_SetAndClear(t *testing.T) {
	settingsHome(t, "# my note\nlanguage: ko\n")
	svc := &SettingsService{}

	for key, js := range map[string]string{
		"speech_threshold":    `0.6`,
		"energy_threshold":    `300`,
		"silence_duration":    `"8s"`,
		"transcript_denylist": `["thanks for watching", "subscribe"]`,
		"initial_prompt":      `"tacit: whisper"`,
	} {
		s, err := svc.Set(key, fromJS(t, js))
		if err != nil {
			t.Fatalf("Set(%s, %s): %v", key, js, err)
		}
		f := field(t, s, key)
		got, _ := json.Marshal(f.Value)
		if !f.Overridden || string(got) != strings.ReplaceAll(js, `", "`, `","`) {
			t.Errorf("after Set(%s, %s): %+v", key, js, f)
		}
	}

	s, err := svc.Clear("language")
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if f := field(t, s, "language"); f.Overridden || f.Value != "auto" {
		t.Errorf("after Clear: %+v", f)
	}
	data, _ := os.ReadFile(configure.OverridePath())
	if !strings.HasPrefix(string(data), "# my note\n") {
		t.Errorf("the user's comment was lost:\n%s", data)
	}
}

// A change setup would reject — a model the provider lacks — loads fine but
// stops `tacit listen` from starting, so it is refused and the file is left
// alone.
func TestSettings_RefusesInvalidChoices(t *testing.T) {
	settingsHome(t, "llm_provider: claude\nllm_model: sonnet\n")
	svc := &SettingsService{}
	before, _ := os.ReadFile(configure.OverridePath())

	if _, err := svc.Clear("llm_model"); err == nil || !strings.Contains(err.Error(), "Claude model") {
		t.Errorf("clearing the Claude model back to the ollama default: err = %v", err)
	}
	if _, err := svc.Set("silence_duration", "30"); err == nil {
		t.Error("a duration without a unit was accepted")
	}
	if after, _ := os.ReadFile(configure.OverridePath()); string(after) != string(before) {
		t.Errorf("a refused change was written:\n%s", after)
	}

	// The same key still changes when the result is valid.
	if _, err := svc.Set("llm_model", "opus"); err != nil {
		t.Errorf("Set(llm_model, opus): %v", err)
	}
}

func TestSettings_LoadReportsBrokenFile(t *testing.T) {
	settingsHome(t, "language: [unclosed\n")
	if _, err := (&SettingsService{}).Load(); err == nil {
		t.Error("Load of an unparsable override file succeeded")
	}
}

// stubDaemon starts a process standing in for `tacit listen`, with its PID in
// a PID file; onTERM is what its shell does on SIGTERM. It is reaped, as the
// app reaps a daemon it started, so an exited one does not linger as a zombie
// that still looks alive.
func stubDaemon(t *testing.T, onTERM string) string {
	t.Helper()
	cmd := exec.Command("sh", "-c", "trap '"+onTERM+"' TERM; while :; do sleep 0.05; done")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go cmd.Wait()
	t.Cleanup(func() { cmd.Process.Kill() })
	pidPath := filepath.Join(t.TempDir(), "tacit.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // let the shell install its trap
	return pidPath
}

// Restart must not start the new daemon until the old one has exited, and the
// old one takes a while: it classifies what it has heard first.
func TestStopAndWait_WaitsForExit(t *testing.T) {
	pidPath := stubDaemon(t, "sleep 0.4; exit 0")
	start := time.Now()
	if err := control.StopAndWait(context.Background(), pidPath, 5*time.Second); err != nil {
		t.Fatalf("stopAndWait: %v", err)
	}
	if took := time.Since(start); took < 400*time.Millisecond {
		t.Errorf("returned after %v, before the daemon exited", took)
	}
	if running, _ := control.Status(pidPath); running {
		t.Error("daemon still running after stopAndWait")
	}
}

func TestStopAndWait_GivesUp(t *testing.T) {
	pidPath := stubDaemon(t, ":") // ignores SIGTERM
	err := control.StopAndWait(context.Background(), pidPath, 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "still stopping") {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestStopAndWait_NotRunning(t *testing.T) {
	if err := control.StopAndWait(context.Background(), filepath.Join(t.TempDir(), "tacit.pid"), time.Second); err == nil {
		t.Error("stopping a daemon that is not running succeeded")
	}
}
