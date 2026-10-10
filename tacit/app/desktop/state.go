package main

import (
	"fmt"
	"path/filepath"
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
	// lastErr describes why a daemon this app started exited on its own.
	lastErr string

	// latest is a release newer than this build, once a check has found one;
	// upToDate says the last check found none.
	latest   string
	upToDate bool
	// checking says a check the user asked for is in flight.
	checking bool
	// updateFailed says the update that reopened the app failed.
	updateFailed bool
}

// observe folds one event into the state.
func (s *state) observe(e control.Event) {
	switch e.Kind {
	case control.KindListening, control.KindSpeechStarted, control.KindSpeechEnded,
		control.KindTranscribing, control.KindClassifying:
		s.activity = e.Kind
	case control.KindTranscribed, control.KindDiscarded, control.KindSkipped, control.KindStored:
		// The segment is done with; classification may still follow, but a
		// transcribed segment waits in a queue until then, so show idle.
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
	switch s.activity {
	case control.KindSpeechStarted:
		return "hearing"
	case control.KindSpeechEnded, control.KindTranscribing, control.KindClassifying:
		return "working"
	default:
		return "listening"
	}
}

// iconFrames counts the images of an animated icon (icons/<name>-<n>.png).
// Two move: the waveform says the app hears you, as a recorder's does, and
// the spinner says it is working on what it heard.
var iconFrames = map[string]int{"hearing": 6, "working": 8}

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
		return fmt.Sprintf("Listening — started from a terminal (PID %d)", s.pid)
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
