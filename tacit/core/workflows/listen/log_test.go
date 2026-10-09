package listen

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWriter_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := NewWriter(path, 0)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	w.Observe(Event{Kind: KindListening, Time: time.Now(), Source: "mic"})
	w.Observe(Event{Kind: KindTranscribed, Time: time.Now(), Source: "mic"})
	w.Observe(Event{Kind: KindStored, Time: time.Now()})

	if err := w.Err(); err != nil {
		t.Fatalf("writer recorded error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d events, want 3", len(got))
	}
	if got[0].Kind != KindListening || got[0].Source != "mic" {
		t.Errorf("event 0 = %+v", got[0])
	}
	if got[1].Kind != KindTranscribed || got[2].Kind != KindStored || got[2].Source != "" {
		t.Errorf("events 1, 2 = %+v, %+v", got[1], got[2])
	}
}

// A front end reads this file to learn what the daemon did; an event whose
// timestamp did not survive the round trip would sort it into the wrong place.
func TestWriter_PreservesTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := NewWriter(path, 0)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	want := time.Date(2026, 3, 14, 15, 9, 26, 0, time.UTC)
	w.Observe(Event{Kind: KindSkipped, Time: want})
	w.Close()

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if !got[0].Time.Equal(want) {
		t.Errorf("Time = %v, want %v", got[0].Time, want)
	}
}

// The daemon appends for weeks, so the log has to be capped. Rotation must not
// lose the events written after it.
func TestWriter_Rotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.ndjson")
	w, err := NewWriter(path, 512)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	for i := 0; i < 40; i++ {
		w.Observe(Event{Kind: KindTranscribed, Time: time.Now(), Source: strings.Repeat("x", 40)})
	}
	if err := w.Err(); err != nil {
		t.Fatalf("writer recorded error: %v", err)
	}
	w.Close()

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected a rotated generation at %s.1: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat live log: %v", err)
	}
	if info.Size() > 512 {
		t.Errorf("live log is %d bytes, want <= 512", info.Size())
	}
	live, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(live) == 0 {
		t.Error("live log is empty after rotation; events written post-rotation were lost")
	}
}

// A daemon killed mid-write leaves a torn final line. Losing the whole history
// to it would make the log useless exactly when something went wrong.
func TestDecode_SkipsTornLine(t *testing.T) {
	in := `{"kind":"listening","time":"2026-03-14T15:09:26Z"}
{"kind":"stored","time":"2026-03-14T15:09:27Z","source":"mic"}
{"kind":"transcri`
	got, err := Decode(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[1].Kind != KindStored || got[1].Source != "mic" {
		t.Errorf("event 1 = %+v", got[1])
	}
}

func TestReadFile_MissingIsEmpty(t *testing.T) {
	got, err := ReadFile(filepath.Join(t.TempDir(), "nope.ndjson"))
	if err != nil {
		t.Fatalf("ReadFile on missing path: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d events, want 0", len(got))
	}
}

// The pipeline emits from one goroutine per capture source plus the classify
// worker, so the writer is genuinely called concurrently.
func TestWriter_ConcurrentObserve(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := NewWriter(path, 0)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				w.Observe(Event{Kind: KindSpeechStarted, Time: time.Now(), Source: "mic"})
			}
		}()
	}
	wg.Wait()
	if err := w.Err(); err != nil {
		t.Fatalf("writer recorded error: %v", err)
	}
	w.Close()

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != 400 {
		t.Errorf("got %d events, want 400 (interleaved writes corrupted the log)", len(got))
	}
}

func TestDiscard_Safe(t *testing.T) {
	Discard.Observe(Event{Kind: KindStored})
}
