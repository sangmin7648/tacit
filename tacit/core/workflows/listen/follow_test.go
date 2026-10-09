package listen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// collector gathers followed events for assertions.
type collector struct {
	mu  sync.Mutex
	got []Event
}

func (c *collector) add(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, e)
}

func (c *collector) snapshot() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.got...)
}

// waitFor polls until at least n events have arrived or the deadline passes.
func (c *collector) waitFor(t *testing.T, n int) []Event {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := c.snapshot(); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := c.snapshot()
	t.Fatalf("got %d events, want %d: %+v", len(got), n, got)
	return nil
}

func startFollow(t *testing.T, path string) *collector {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := &collector{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := Follow(ctx, path, 5*time.Millisecond, c.add); err != nil {
			t.Errorf("Follow: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	// Give Follow time to open the file and seek to its end before the test
	// writes anything it expects to see.
	time.Sleep(30 * time.Millisecond)
	return c
}

// stored returns a stored event labelled by its Source, so a test can tell
// which write it is.
func stored(label string) Event {
	return Event{Kind: KindStored, Time: time.Now(), Source: label}
}

// A front end opening mid-run wants what happens next; history is ReadFile's.
func TestFollow_OnlyNewEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := NewWriter(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Observe(stored("before"))

	c := startFollow(t, path)
	w.Observe(stored("after 1"))
	w.Observe(stored("after 2"))

	got := c.waitFor(t, 2)
	if got[0].Source != "after 1" || got[1].Source != "after 2" {
		t.Errorf("got %q, %q; want the two events written after Follow began", got[0].Source, got[1].Source)
	}
	time.Sleep(30 * time.Millisecond)
	if n := len(c.snapshot()); n != 2 {
		t.Errorf("got %d events, want exactly 2 (history must not be replayed)", n)
	}
}

// The app may start before the daemon has ever run.
func TestFollow_LogCreatedLater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	c := startFollow(t, path)

	w, err := NewWriter(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Observe(stored("first ever"))

	if got := c.waitFor(t, 1); got[0].Source != "first ever" {
		t.Errorf("got %q", got[0].Source)
	}
}

// Rotation moves the live file aside mid-stream. Everything written on either
// side of it must arrive, in order.
func TestFollow_SurvivesRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := NewWriter(path, 600) // a handful of events per file
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	c := startFollow(t, path)

	const n = 12
	for i := 0; i < n; i++ {
		w.Observe(stored(fmt.Sprintf("entry %02d", i)))
		// Pace the writer so each rotation is seen before the next one: two
		// rotations between polls would overwrite .1 before it was drained.
		time.Sleep(15 * time.Millisecond)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("test did not rotate the log: %v", err)
	}

	got := c.waitFor(t, n)
	for i, e := range got[:n] {
		if want := fmt.Sprintf("entry %02d", i); e.Source != want {
			t.Fatalf("event %d = %q, want %q (lost or reordered across rotation)", i, e.Source, want)
		}
	}
}

// A poll can land mid-write. The half line must wait for its end, not be
// dropped or parsed twice.
func TestFollow_PartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c := startFollow(t, path)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString(`{"kind":"stored","time":"2026-09-27T16:00:00Z",`)
	time.Sleep(30 * time.Millisecond)
	if n := len(c.snapshot()); n != 0 {
		t.Fatalf("delivered %d events from half a line", n)
	}
	f.WriteString(`"source":"joined"}` + "\n")

	if got := c.waitFor(t, 1); got[0].Source != "joined" {
		t.Errorf("got %+v", got[0])
	}
}

func TestFollow_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Follow(ctx, filepath.Join(t.TempDir(), "x"), time.Millisecond, func(Event) {}) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Follow returned %v on cancel, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Follow did not return after cancel")
	}
}
