package main

import (
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/sangmin7648/tacit/pkg/events"
	"github.com/sangmin7648/tacit/pkg/storage"
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
	activity events.Kind
	// recent holds the newest stored entries, newest first.
	recent []*storage.KnowledgeEntry
	// lastErr describes why a daemon this app started exited on its own.
	lastErr string

	// latest is a release newer than this build, once a check has found one;
	// upToDate says the last check found none.
	latest   string
	upToDate bool
	// updateFailed says the update that reopened the app failed.
	updateFailed bool
}

// observe folds one event into the state.
func (s *state) observe(e events.Event) {
	switch e.Kind {
	case events.KindListening, events.KindSpeechStarted, events.KindSpeechEnded,
		events.KindTranscribing, events.KindClassifying:
		s.activity = e.Kind
	case events.KindTranscribed, events.KindDiscarded, events.KindSkipped, events.KindStored:
		// The segment is done with; classification may still follow, but a
		// transcribed segment waits in a queue until then, so show idle.
		s.activity = events.KindListening
	}
}

// label is the menu-bar text: one glyph for what the daemon is doing.
func (s *state) label() string {
	if !s.running {
		return "○"
	}
	switch s.activity {
	case events.KindSpeechStarted:
		return "◉" // hearing speech
	case events.KindSpeechEnded, events.KindTranscribing, events.KindClassifying:
		return "◐" // working on what it heard
	default:
		return "●" // listening
	}
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
func entryLabel(e *storage.KnowledgeEntry) string {
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
