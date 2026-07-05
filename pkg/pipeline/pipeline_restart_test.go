package pipeline

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/capture"
	"github.com/sangmin7648/tacit/pkg/config"
)

// fakeSource is a scripted capture.AudioSource for exercising the runSource
// restart logic without real audio hardware.  streamFn decides what each
// successive Stream() call returns (error to simulate re-init failure, or a
// channel to feed / stall).
type fakeSource struct {
	mu       sync.Mutex
	calls    int
	closed   int
	streamFn func(ctx context.Context, call int) (<-chan []int16, error)
}

var _ capture.AudioSource = (*fakeSource)(nil)

func (f *fakeSource) Stream(ctx context.Context) (<-chan []int16, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	fn := f.streamFn
	f.mu.Unlock()
	return fn(ctx, call)
}

func (f *fakeSource) Close() {
	f.mu.Lock()
	f.closed++
	f.mu.Unlock()
}

func (f *fakeSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// withTimings shrinks the package-level retry/stall timings for the duration of
// a test and restores them afterwards.
func withTimings(t *testing.T, retry, stall time.Duration) {
	t.Helper()
	origRetry, origStall := retryDelay, stallTimeout
	retryDelay, stallTimeout = retry, stall
	t.Cleanup(func() { retryDelay, stallTimeout = origRetry, origStall })
}

// TestRunSourceRecoversFromReinitFailure is the regression test for the primary
// bug: after sleep/wake, speaker_create fails transiently and the old code gave
// up permanently.  runSource must keep retrying until Stream() succeeds.
func TestRunSourceRecoversFromReinitFailure(t *testing.T) {
	withTimings(t, 5*time.Millisecond, 10*time.Second)
	p := &Pipeline{cfg: config.DefaultConfig()}

	const failN = 3
	firstSuccess := make(chan struct{})
	src := &fakeSource{}
	src.streamFn = func(ctx context.Context, call int) (<-chan []int16, error) {
		if call <= failN {
			return nil, fmt.Errorf("simulated speaker_create failure #%d", call)
		}
		if call == failN+1 {
			close(firstSuccess)
		}
		// Success: an open channel that yields no data and is closed on cancel.
		ch := make(chan []int16)
		go func() {
			<-ctx.Done()
			close(ch)
		}()
		return ch, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	classifyCh := make(chan classifyItem, 4)
	done := make(chan struct{})
	go func() { _ = p.runSource(ctx, src, "test", classifyCh); close(done) }()

	select {
	case <-firstSuccess:
		// Recovered after transient failures — the bug is fixed.
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatalf("runSource gave up instead of retrying; Stream called %d times", src.callCount())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSource did not exit after ctx cancel")
	}
	if got := src.callCount(); got < failN+1 {
		t.Fatalf("expected at least %d Stream() calls, got %d", failN+1, got)
	}
}

// TestRunSourceRestartsOnStall covers Defect B: a stream that stops delivering
// audio without closing its channel (SCStream torn down on sleep with no stop
// callback) must be detected by the inactivity watchdog and restarted.
func TestRunSourceRestartsOnStall(t *testing.T) {
	withTimings(t, 5*time.Millisecond, 80*time.Millisecond)
	p := &Pipeline{cfg: config.DefaultConfig()}

	restarted := make(chan struct{}, 1)
	src := &fakeSource{}
	src.streamFn = func(ctx context.Context, call int) (<-chan []int16, error) {
		if call >= 2 {
			select {
			case restarted <- struct{}{}:
			default:
			}
		}
		// Deliver two silent chunks, then go silent forever (never close).
		ch := make(chan []int16, 4)
		ch <- make([]int16, 256)
		ch <- make([]int16, 256)
		return ch, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	classifyCh := make(chan classifyItem, 4)
	done := make(chan struct{})
	go func() { _ = p.runSource(ctx, src, "test", classifyCh); close(done) }()

	select {
	case <-restarted:
		// Watchdog fired and the source was restarted.
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatalf("watchdog did not restart stalled stream; Stream called %d times", src.callCount())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSource did not exit after ctx cancel")
	}
}

// TestRunSourceExitsOnCancel ensures the new select-based loop still returns
// promptly on ctx cancellation, well before the (long) watchdog timeout.
func TestRunSourceExitsOnCancel(t *testing.T) {
	withTimings(t, 5*time.Millisecond, 10*time.Second)
	p := &Pipeline{cfg: config.DefaultConfig()}

	src := &fakeSource{}
	src.streamFn = func(ctx context.Context, call int) (<-chan []int16, error) {
		// Open channel, no data, never closed — only ctx cancel can end it.
		return make(chan []int16), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	classifyCh := make(chan classifyItem, 4)
	done := make(chan struct{})
	go func() { _ = p.runSource(ctx, src, "test", classifyCh); close(done) }()

	time.Sleep(30 * time.Millisecond) // let it settle into the select loop
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("runSource did not exit promptly on ctx cancel (watchdog timeout is 10s)")
	}
}
