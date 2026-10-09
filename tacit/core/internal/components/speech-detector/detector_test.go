package speechdetector

import (
	"testing"
	"time"
)

// Silence must never open a segment, and the buffer of partial frames must
// not grow with the stream.
func TestDetector_SilenceYieldsNothing(t *testing.T) {
	d, err := NewDetector(Options{SpeechThreshold: 0.5, MinSpeech: time.Second, Silence: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	chunk := make([]int16, 1000) // not a multiple of the VAD frame
	for i := 0; i < 100; i++ {
		if events := d.Feed(chunk); len(events) != 0 {
			t.Fatalf("chunk %d: got %v from silence", i, events)
		}
	}
	if len(d.pending) >= hopSize {
		t.Errorf("pending holds %d samples, want less than one frame", len(d.pending))
	}
}
