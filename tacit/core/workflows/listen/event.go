// Events tell a front end what a running daemon is doing right now, so the
// menu bar can show it and refresh its notes when one is stored. The daemon
// runs as its own process, so the front end has no other way to know.
//
// Events carry only what a front end displays. Why a segment was dropped, and
// what was said, belong in the daemon log, which is the record a person reads
// to diagnose a run.

package listen

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
	// KindTranscribed reports that whisper returned text worth classifying.
	KindTranscribed Kind = "transcribed"
	// KindDiscarded reports speech dropped before classification — too short,
	// filler, hallucinated boilerplate, or a stock repeat.
	KindDiscarded Kind = "discarded"
	// KindClassifying reports a classify call starting.
	KindClassifying Kind = "classifying"
	// KindStored reports a knowledge entry written to disk.
	KindStored Kind = "stored"
	// KindSkipped reports a transcript the classifier called meaningless.
	KindSkipped Kind = "skipped"
)

// Event is one observable moment.
type Event struct {
	Kind Kind      `json:"kind"`
	Time time.Time `json:"time"`

	// Source is the capture source the event came from ("mic"), or
	// empty for events that belong to the daemon as a whole.
	Source string `json:"source,omitempty"`
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
