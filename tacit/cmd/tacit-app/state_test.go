package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/events"
)

func TestLabel_FollowsActivity(t *testing.T) {
	s := &state{}
	if got := s.label(); got != "○" {
		t.Errorf("stopped label = %q", got)
	}
	s.running = true
	steps := []struct {
		kind events.Kind
		want string
	}{
		{events.KindListening, "●"},
		{events.KindSpeechStarted, "◉"},
		{events.KindSpeechEnded, "◐"},
		{events.KindTranscribing, "◐"},
		{events.KindTranscribed, "●"},
		{events.KindClassifying, "◐"},
		{events.KindStored, "●"},
		{events.KindSpeechStarted, "◉"},
		{events.KindDiscarded, "●"},
		{events.KindClassifying, "◐"},
		{events.KindSkipped, "●"},
		{events.KindError, "●"}, // an absorbed error does not change what it is doing
	}
	for i, st := range steps {
		s.observe(events.Event{Kind: st.kind})
		if got := s.label(); got != st.want {
			t.Errorf("step %d (%s): label = %q, want %q", i, st.kind, got, st.want)
		}
	}
}

func TestRecent_NewestFirstAndCapped(t *testing.T) {
	s := &state{}
	var history []events.Event
	for i := 0; i < recentLimit+3; i++ {
		history = append(history, events.Event{Kind: events.KindStored, Title: fmt.Sprintf("e%d", i)})
		history = append(history, events.Event{Kind: events.KindTranscribed, Text: "noise"})
	}
	s.seedRecent(history)

	if len(s.recent) != recentLimit {
		t.Fatalf("recent has %d entries, want %d", len(s.recent), recentLimit)
	}
	if s.recent[0].Title != fmt.Sprintf("e%d", recentLimit+2) {
		t.Errorf("recent[0] = %q, want the newest", s.recent[0].Title)
	}
	if s.activity != "" {
		t.Errorf("history left activity = %q; a replay says nothing about a daemon running now", s.activity)
	}

	s.observe(events.Event{Kind: events.KindStored, Title: "live"})
	if s.recent[0].Title != "live" || len(s.recent) != recentLimit {
		t.Errorf("after a live store: %q first, %d total", s.recent[0].Title, len(s.recent))
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
		e    events.Event
		want string
	}{
		{events.Event{Title: "검색 랭킹 논의", Category: "work"}, "work · 검색 랭킹 논의"},
		{events.Event{Path: "/x/daily/20260927-161254.md"}, "20260927-161254.md"},
		{events.Event{Title: strings.Repeat("가", 60)}, strings.Repeat("가", 47) + "…"},
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
		if running, pid := daemonStatus(pidPath); running {
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

	if err := stopDaemon(pidPath); err != nil {
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
	if running, _ := daemonStatus(pidPath); running {
		t.Error("still reported running after stop")
	}
	if err := stopDaemon(pidPath); err == nil {
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
