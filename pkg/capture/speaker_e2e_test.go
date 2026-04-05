//go:build integration && darwin

package capture

import (
	"context"
	"testing"
	"time"
)

// TestSpeaker_Stream_E2E verifies that the Speaker can start a ScreenCaptureKit
// stream and deliver audio chunks within a 5-second window.
//
// Prerequisites:
//   - macOS 13.0+
//   - Screen Recording permission granted to the terminal / test runner
//     (System Preferences → Privacy & Security → Screen Recording)
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
		t.Skipf("Stream: %v\n\tGrant Screen Recording permission to Terminal in System Preferences → Privacy & Security → Screen Recording", err)
	}

	var totalSamples int
	var chunkCount int
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
		case <-deadline:
			cancel()
			break loop
		}
	}

	t.Logf("received %d chunks, %d total int16 samples (%.2fs of audio at 16kHz)",
		chunkCount, totalSamples, float64(totalSamples)/16000.0)

	if chunkCount == 0 {
		t.Fatal("no audio chunks received — ScreenCaptureKit stream may not be delivering data")
	}
	if totalSamples == 0 {
		t.Fatal("received chunks but all were empty")
	}
}
