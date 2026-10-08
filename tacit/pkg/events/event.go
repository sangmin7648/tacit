// Package events carries the pipeline's observable moments to anything that
// wants to watch a running daemon — today the CLI's own log, tomorrow the Mac
// app's menu bar and activity feed.
//
// The pipeline already narrated itself through log.Printf, but only to a human
// reading stderr: a second front end would have had to scrape English prose to
// learn that a segment was stored. An Event is that same narration as data.
// Emitting one never replaces the log line it accompanies.
package events

import "time"

// Kind names the moment an Event reports. Kinds are additive: a reader that
// does not recognise one must ignore it rather than fail, so a newer daemon
// stays readable by an older front end.
type Kind string

const (
	// KindListening reports that a capture source has started a session and is
	// waiting for speech. It repeats on every automatic restart.
	KindListening Kind = "listening"
	// KindSpeechStarted reports VAD's speech onset.
	KindSpeechStarted Kind = "speech_started"
	// KindSpeechEnded reports that silence closed a speech segment.
	KindSpeechEnded Kind = "speech_ended"
	// KindTranscribing reports that a segment was handed to whisper. It also
	// fires for a split of ongoing speech, so it does not always follow a
	// KindSpeechEnded.
	KindTranscribing Kind = "transcribing"
	// KindTranscribed carries the text whisper returned and kept.
	KindTranscribed Kind = "transcribed"
	// KindDiscarded reports speech dropped before classification — filler,
	// hallucinated boilerplate, too sparse for its duration, or a stock repeat.
	// Reason says which.
	KindDiscarded Kind = "discarded"
	// KindClassifying reports a classify call starting. Count is how many
	// transcripts went into it.
	KindClassifying Kind = "classifying"
	// KindStored reports a knowledge entry written to disk.
	KindStored Kind = "stored"
	// KindSkipped reports a transcript the classifier called meaningless. It is
	// the one path that intentionally throws speech away.
	KindSkipped Kind = "skipped"
	// KindError reports a failure the daemon absorbed and continued past.
	KindError Kind = "error"
)

// Event is one observable moment. Every field but Kind and Time is optional,
// and which ones are populated depends on Kind; a reader should treat a missing
// field as unknown rather than as an empty value that means something.
type Event struct {
	Kind Kind      `json:"kind"`
	Time time.Time `json:"time"`

	// Source is the capture source the event came from ("mic"), or
	// empty for events that belong to the daemon as a whole.
	Source string `json:"source,omitempty"`

	// Text is the transcript an event concerns, for the kinds that have one.
	Text string `json:"text,omitempty"`

	// Seconds is the audio duration an event concerns, for the kinds that have
	// one. It is audio length, never wall-clock elapsed time.
	Seconds float64 `json:"seconds,omitempty"`

	// Count is how many transcripts a classify call covers.
	Count int `json:"count,omitempty"`

	// Path, Title and Category describe a stored knowledge entry.
	Path     string `json:"path,omitempty"`
	Title    string `json:"title,omitempty"`
	Category string `json:"category,omitempty"`

	// Reason explains a discard or a restart in a short, stable token.
	Reason string `json:"reason,omitempty"`

	// Error is the message of a failure the daemon continued past.
	Error string `json:"error,omitempty"`
}

// Observer receives pipeline events. Observe is called from the pipeline's own
// goroutines — several of them concurrently, one per capture source plus the
// classify worker — so an implementation must be safe for concurrent use and
// must return promptly: it runs inline with audio processing, and anything
// slow enough to block belongs behind the implementation's own buffer.
type Observer interface {
	Observe(Event)
}

// ObserverFunc adapts a plain function to Observer.
type ObserverFunc func(Event)

// Observe calls f.
func (f ObserverFunc) Observe(e Event) { f(e) }

// Discard is an Observer that drops everything. It is the pipeline's default,
// so emitting is always safe even when nothing is watching.
var Discard Observer = ObserverFunc(func(Event) {})
