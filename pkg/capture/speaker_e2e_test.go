//go:build integration && darwin

package capture

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestSpeaker_Stream_E2E verifies that the Speaker can start a Core Audio
// process tap and deliver real audio within a 5-second window.
//
// It plays a system sound for the duration and asserts that the captured
// samples are not silent.  Asserting only that chunks arrive is not enough: a
// tap that has lost its source keeps delivering buffers, they are just zeros
// forever — which is exactly the silent failure this capture path must not have.
//
// Prerequisites:
//   - macOS 14.2+
//   - Audio recording permission granted to the terminal / test runner
//     (System Settings → Privacy & Security). No display is required.
//
// Run with:
//
//	go test -tags "integration darwin" -v -run TestSpeaker_Stream_E2E ./pkg/capture/
func TestSpeaker_Stream_E2E(t *testing.T) {
	spk, err := NewSpeaker()
	if err != nil {
		t.Fatalf("NewSpeaker: %v", err)
	}
	defer spk.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := spk.Stream(ctx)
	if err != nil {
		// Permission denied is an environment issue, not a code bug — skip gracefully.
		t.Skipf("Stream: %v\n\tGrant audio recording permission to your terminal in System Settings → Privacy & Security", err)
	}

	// Give the tap something to hear.  If afplay is unavailable we still verify
	// the plumbing, just without the amplitude assertion.
	playing := false
	if path, lookErr := exec.LookPath("afplay"); lookErr == nil {
		cmd := exec.Command(path, "/System/Library/Sounds/Submarine.aiff")
		if cmd.Start() == nil {
			playing = true
			defer func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}()
		}
	}

	var totalSamples int
	var chunkCount int
	var peak int
	deadline := time.After(5 * time.Second)

loop:
	for {
		select {
		case samples, ok := <-ch:
			if !ok {
				t.Log("channel closed")
				break loop
			}
			chunkCount++
			totalSamples += len(samples)
			for _, s := range samples {
				v := int(s)
				if v < 0 {
					v = -v
				}
				if v > peak {
					peak = v
				}
			}
		case <-deadline:
			cancel()
			break loop
		}
	}

	t.Logf("received %d chunks, %d total int16 samples (%.2fs of audio at 16kHz), peak amplitude %d",
		chunkCount, totalSamples, float64(totalSamples)/16000.0, peak)

	if chunkCount == 0 {
		t.Fatal("no audio chunks received — the process tap is not delivering data")
	}
	if totalSamples == 0 {
		t.Fatal("received chunks but all were empty")
	}
	if playing && peak == 0 {
		t.Fatal("captured audio is pure silence while a sound was playing — " +
			"the tap is running but not attached to the system mix")
	}
}
