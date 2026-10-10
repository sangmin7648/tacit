package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/core/workflows/control"

	"github.com/sangmin7648/tacit/core/workflows/browse"
)

func TestIcon_FollowsActivity(t *testing.T) {
	s := &state{}
	if got := s.icon(); got != "off" {
		t.Errorf("stopped icon = %q", got)
	}
	s.running = true
	steps := []struct {
		kind control.Kind
		want string
	}{
		{control.KindListening, "listening"},
		{control.KindSpeechStarted, "hearing"},
		{control.KindSpeechEnded, "working"},
		{control.KindTranscribing, "working"},
		{control.KindTranscribed, "listening"},
		{control.KindClassifying, "working"},
		{control.KindStored, "listening"},
		{control.KindSpeechStarted, "hearing"},
		{control.KindDiscarded, "listening"},
		{control.KindClassifying, "working"},
		{control.KindSkipped, "listening"},
	}
	for i, st := range steps {
		s.observe(control.Event{Kind: st.kind})
		if got := s.icon(); got != st.want {
			t.Errorf("step %d (%s): icon = %q, want %q", i, st.kind, got, st.want)
		}
	}
}

// An error outranks activity, running or not: it is the one state the user has
// to act on.
func TestIcon_ErrorOutranksActivity(t *testing.T) {
	for _, running := range []bool{false, true} {
		s := &state{running: running, lastErr: "Couldn't start: boom", activity: control.KindSpeechStarted}
		if got := s.icon(); got != "error" {
			t.Errorf("running=%v: icon = %q, want error", running, got)
		}
	}
}

// Every icon a state can name must ship, as a PNG the tray can load.
func TestTrayIcons_AllShip(t *testing.T) {
	for _, name := range []string{"off", "listening", "hearing", "working", "error"} {
		for tick := range max(iconFrames[name], 1) {
			file := iconFile(name, tick)
			data, err := trayIcons.ReadFile(file)
			if err != nil {
				t.Errorf("%s: %v", file, err)
				continue
			}
			if !strings.HasPrefix(string(data), "\x89PNG") {
				t.Errorf("%s is not a PNG", file)
			}
		}
	}
	if a, b := iconFile("hearing", 0), iconFile("hearing", iconFrames["hearing"]); a != b {
		t.Errorf("hearing does not loop: %s then %s", a, b)
	}
	if iconFile("listening", 0) != iconFile("listening", 9) {
		t.Error("a still icon changed between ticks")
	}
}

func TestStatusLine_SaysWhoStartedIt(t *testing.T) {
	if got := (&state{}).statusLine(); got != "Not listening" {
		t.Errorf("stopped: %q", got)
	}
	own := &state{running: true, pid: 42, ownPID: 42}
	if got := own.statusLine(); got != "Listening (PID 42)" {
		t.Errorf("own: %q", got)
	}
	adopted := &state{running: true, pid: 42}
	if got := adopted.statusLine(); !strings.Contains(got, "terminal") {
		t.Errorf("adopted: %q, want it to say it came from a terminal", got)
	}
}

func TestEntryLabel(t *testing.T) {
	cases := []struct {
		e    *browse.Note
		want string
	}{
		{&browse.Note{Title: "검색 랭킹 논의", Category: "work"}, "work · 검색 랭킹 논의"},
		{&browse.Note{FilePath: "/x/daily/20260927-161254.md"}, "20260927-161254.md"},
		{&browse.Note{Title: strings.Repeat("가", 60)}, strings.Repeat("가", 47) + "…"},
	}
	for _, c := range cases {
		if got := entryLabel(c.e); got != c.want {
			t.Errorf("entryLabel(%+v) = %q, want %q", c.e, got, c.want)
		}
	}
}

// spawnListen and stopDaemon against a stand-in CLI: a script that behaves
// like `tacit listen` — writes the PID file, runs until SIGTERM, removes it.
// It writes the PID file last, so the test, which signals as soon as the file
// appears, cannot catch it before its trap is set.
func TestSpawnAndStop(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "tacit.pid")
	logPath := filepath.Join(dir, "daemon.log")
	cli := filepath.Join(dir, "tacit")
	script := fmt.Sprintf(`#!/bin/sh
[ "$1" = listen ] || exit 2
trap 'rm -f %q; echo stopped; exit 0' TERM
echo listening
echo $$ > %q
while :; do sleep 0.05; done
`, pidPath, pidPath)
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(logPath, []byte("previous run\n"), 0o644)

	cmd, err := spawnListen(cli, logPath)
	if err != nil {
		t.Fatalf("spawnListen: %v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	deadline := time.Now().Add(3 * time.Second)
	for {
		if running, pid := control.Status(pidPath); running {
			if pid != cmd.Process.Pid {
				t.Fatalf("PID file says %d, spawned %d", pid, cmd.Process.Pid)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stand-in daemon never wrote its PID file")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := control.Stop(pidPath); err != nil {
		t.Fatalf("stopDaemon: %v", err)
	}
	select {
	case err := <-waited:
		if err != nil {
			t.Errorf("daemon exited with %v after SIGTERM, want a clean exit", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit after stopDaemon")
	}
	if running, _ := control.Status(pidPath); running {
		t.Error("still reported running after stop")
	}
	if err := control.Stop(pidPath); err == nil {
		t.Error("stopDaemon on a stopped daemon returned nil")
	}

	out, _ := os.ReadFile(logPath)
	if got := string(out); got != "listening\nstopped\n" {
		t.Errorf("daemon log = %q, want this run's output only", got)
	}
}

// A daemon the app started stays the app's across app runs — so Quit stops
// it — while one started from a terminal never becomes the app's.
func TestAdopt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(ownerPath()), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := adopt(true, 4242); got != 0 {
		t.Errorf("with no record, adopt = %d, want 0 (a terminal's daemon)", got)
	}
	if err := recordOwned(4242); err != nil {
		t.Fatal(err)
	}
	if got := adopt(true, 4242); got != 4242 {
		t.Errorf("adopt = %d, want the recorded 4242", got)
	}
	if got := adopt(true, 5151); got != 0 {
		t.Errorf("a different running daemon was adopted: %d", got)
	}
	if got := adopt(false, 4242); got != 0 {
		t.Errorf("a daemon that is not running was adopted: %d", got)
	}

	clearOwned(5151) // another daemon stopping leaves the record alone
	if ownedPID() != 4242 {
		t.Error("clearing another PID dropped the record")
	}
	clearOwned(4242)
	if ownedPID() != 0 {
		t.Error("the record survived its daemon stopping")
	}

	os.WriteFile(ownerPath(), []byte("garbage"), 0o644)
	if got := adopt(true, 0); got != 0 {
		t.Errorf("an unreadable record adopted PID %d", got)
	}
}

func TestExplainExit_NamesTheCauseAndTheFix(t *testing.T) {
	for _, c := range []struct {
		failure  string
		contains string
		fix      fix
	}{
		{"2026/10/10 tacit listen: initializing pipeline: ensure whisper model: no such file", "Speech model", fixNone},
		{"2026/10/10 tacit listen: init whisper: failed to load whisper model from /x", "Speech model", fixNone},
		{"2026/10/10 tacit listen: initializing microphone: init audio context: boom", "microphone", fixMicrophone},
		{"2026/10/10 tacit listen: start stream: start capture: denied", "microphone", fixMicrophone},
		{"2026/10/10 tacit listen: init speech detector: x", "stopped", fixNone},
		{"", "stopped", fixNone},
	} {
		msg, f := explainExit(c.failure)
		if !strings.Contains(msg, c.contains) || f != c.fix {
			t.Errorf("explainExit(%q) = %q, %v; want %q, %v", c.failure, msg, f, c.contains, c.fix)
		}
	}
}

func TestLastFailure_FindsTheExitReason(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	os.WriteFile(path, []byte("whisper_model_load: type = 5\n2026/10/10 19:08:39 tacit listen: init whisper: failed\n"), 0o644)
	if got := lastFailure(path); !strings.Contains(got, "init whisper: failed") {
		t.Errorf("lastFailure = %q", got)
	}
	os.WriteFile(path, []byte("tacit daemon started\n"), 0o644)
	if got := lastFailure(path); got != "" {
		t.Errorf("a log with no failure gave %q", got)
	}
	if got := lastFailure(filepath.Join(t.TempDir(), "missing.log")); got != "" {
		t.Errorf("a missing log gave %q", got)
	}
}

func TestFail_ClearsOnSuccess(t *testing.T) {
	s := &state{}
	s.fail("x", fixMicrophone)
	if s.icon() != "error" {
		t.Error("a failure did not show the error icon")
	}
	s.clearFailure()
	if s.lastErr != "" || s.lastFix != fixNone || s.icon() == "error" {
		t.Errorf("failure not cleared: %+v", s)
	}
}
