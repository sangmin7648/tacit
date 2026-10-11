package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/sangmin7648/tacit/core/workflows/control"

	"github.com/sangmin7648/tacit/core/workflows/browse"
)

// recentLimit is how many stored entries the menu lists.
const recentLimit = 5

// state is everything the menu shows. It is built from three sources: the PID
// file, which says whether a daemon is running (whoever started it), the event
// log, which says what that daemon is doing, and the notes folder.
type state struct {
	running bool
	pid     int
	// ownPID is the daemon the app started — this run or an earlier one that
	// crashed or was killed (see adopt) — or 0. A daemon started from a
	// terminal is shown and can be stopped here, but quitting the app leaves
	// it alone; the app's own stops when the app quits.
	ownPID int
	// activity is the latest event kind from the running daemon.
	activity control.Kind
	// recent holds the newest stored entries, newest first.
	recent []*browse.Note
	// lastErr describes what went wrong, in words the user can act on; lastFix
	// is the action the menu offers beside it.
	lastErr string
	lastFix fix

	// latest is a release newer than this build, once a check has found one;
	// upToDate says the last check found none.
	latest   string
	upToDate bool
	// checking says a check the user asked for is in flight.
	checking bool
	// updateFailed says the update that reopened the app failed.
	updateFailed bool
}

// fix is an action the menu offers beside an error. Errors whose answer is a
// menu item that is always there (Set Up Tacit…, Open Daemon Log) just name it.
type fix int

const (
	fixNone fix = iota
	fixMicrophone
)

func (s *state) fail(msg string, f fix) { s.lastErr, s.lastFix = msg, f }
func (s *state) clearFailure()          { s.lastErr, s.lastFix = "", fixNone }

// mustStopForMicrophone reports whether the daemon this app started is running
// with the microphone denied. It hears only silence then, and nothing else
// would say so. A terminal's daemon has the terminal's permission, not the
// app's, and an error already shown is not shown twice. The status is asked
// for only when it matters.
func mustStopForMicrophone(s *state, status func() string) bool {
	own := s.running && s.ownPID != 0 && s.ownPID == s.pid
	return own && s.lastFix != fixMicrophone && status() == permDenied
}

// explainExit turns a daemon that exited on its own into an error the user can
// act on. failure is the daemon's last "tacit listen: ..." log line: the one
// that says why.
func explainExit(failure string) (string, fix) {
	f := strings.ToLower(failure)
	switch {
	case strings.Contains(f, "microphone") || strings.Contains(f, "capture") || strings.Contains(f, "start stream"):
		return "Can't use the microphone", fixMicrophone
	case strings.Contains(f, "whisper") || strings.Contains(f, "model"):
		return "Speech model not found", fixNone
	default:
		return "Tacit stopped unexpectedly", fixNone
	}
}

// lastFailure returns the last line of the daemon log that reports why it
// exited, or "".
func lastFailure(logPath string) string {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "tacit listen: ") {
			return lines[i]
		}
	}
	return ""
}

// observe folds one event into the state. Only hearing speech is shown:
// what happens to a segment afterwards (transcribing, classifying, a queue of
// both) is not something the user can act on, so the icon returns to listening.
func (s *state) observe(e control.Event) {
	switch e.Kind {
	case control.KindSpeechStarted:
		s.activity = e.Kind
	case control.KindListening, control.KindSpeechEnded, control.KindTranscribing, control.KindClassifying,
		control.KindTranscribed, control.KindDiscarded, control.KindSkipped, control.KindStored:
		s.activity = control.KindListening
	}
}

// icon names the menu-bar image (icons/<name>.png), one per thing the user
// needs to tell apart. The shapes differ, not only the colour: a template
// image has no colour, and the states must read for everyone.
func (s *state) icon() string {
	switch {
	case s.lastErr != "":
		return "error"
	case !s.running:
		return "off"
	}
	if s.activity == control.KindSpeechStarted {
		return "hearing"
	}
	return "listening"
}

// iconFrames counts the images of an animated icon (icons/<name>-<n>.png).
// Only the waveform moves: it says the app hears you, as a recorder's does.
var iconFrames = map[string]int{"hearing": 6}

// iconFile is the image for icon name at animation step tick.
func iconFile(name string, tick int) string {
	if n := iconFrames[name]; n > 0 {
		return fmt.Sprintf("icons/%s-%d.png", name, tick%n)
	}
	return "icons/" + name + ".png"
}

// statusLine is the menu's first, disabled item.
func (s *state) statusLine() string {
	switch {
	case !s.running:
		return "Not listening"
	case s.ownPID != 0 && s.ownPID == s.pid:
		return fmt.Sprintf("Listening (PID %d)", s.pid)
	default:
		return fmt.Sprintf("Listening (from a terminal, PID %d)", s.pid)
	}
}

// entryLabel is how a stored entry reads in the Recent list.
func entryLabel(e *browse.Note) string {
	title := e.Title
	if title == "" {
		title = filepath.Base(e.FilePath)
	}
	const max = 48
	if utf8.RuneCountInString(title) > max {
		r := []rune(title)
		title = string(r[:max-1]) + "…"
	}
	if e.Category != "" {
		return e.Category + " · " + title
	}
	return title
}
