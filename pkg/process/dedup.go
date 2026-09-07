package process

import (
	"hash/fnv"
	"sync"
	"time"
)

// TranscriptDeduper flags a transcript whose normalised text has already been
// seen several times inside a recent window.
//
// A whisper stock hallucination — "감사합니다.", a subtitle notice, a news
// sign-off — reappears verbatim far more often than genuine speech ever does:
// the same normalised transcript landing dozens of times a day is itself the
// signal, and it is one no single-item filter or classifier prompt can see.
// The first keepFirst occurrences in the window are always let through, so a
// real remark that happens to recur is never lost.
type TranscriptDeduper struct {
	window    time.Duration
	keepFirst int

	mu   sync.Mutex
	seen []seenTranscript
}

type seenTranscript struct {
	hash uint64
	at   time.Time
}

// NewTranscriptDeduper returns a deduper that lets the first keepFirst verbatim
// occurrences through per rolling window and flags the rest. A window <= 0 or
// keepFirst <= 0 disables it — Seen then always reports (0, false), as does a
// nil *TranscriptDeduper.
func NewTranscriptDeduper(window time.Duration, keepFirst int) *TranscriptDeduper {
	return &TranscriptDeduper{window: window, keepFirst: keepFirst}
}

// Seen records that text was produced at now and reports how many matching
// occurrences were already inside the window (prior) and whether this one
// should be treated as a stock repeat (prior >= keepFirst). now is a parameter
// rather than time.Now() so the pipeline can pass the transcript's own
// timestamp and tests can drive the clock.
func (d *TranscriptDeduper) Seen(text string, now time.Time) (prior int, repeat bool) {
	if d == nil || d.window <= 0 || d.keepFirst <= 0 {
		return 0, false
	}
	key := normalizeForMatch(text)
	if key == "" {
		return 0, false
	}
	h := fnv.New64a()
	h.Write([]byte(key))
	sum := h.Sum64()

	cutoff := now.Add(-d.window)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Prune entries that have aged out of the window, counting live matches on
	// the same pass.
	kept := d.seen[:0]
	for _, e := range d.seen {
		if e.at.Before(cutoff) {
			continue
		}
		kept = append(kept, e)
		if e.hash == sum {
			prior++
		}
	}
	d.seen = append(kept, seenTranscript{hash: sum, at: now})

	return prior, prior >= d.keepFirst
}
