package audio

import (
	"os/exec"
	"testing"
)

func TestDecodeFile_M4A(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping decode test")
	}

	samples, err := DecodeFile("../../testdata/test_voice_recording.m4a")
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	if len(samples) == 0 {
		t.Fatal("expected non-empty samples")
	}

	// Verify samples are in valid float32 range [-1.0, 1.0]
	for i, s := range samples {
		if s < -1.0 || s > 1.0 {
			t.Fatalf("sample %d out of range: %f", i, s)
		}
	}

	// At 16kHz, even a short recording should have many samples
	expectedMinSamples := 1000 // ~62ms minimum
	if len(samples) < expectedMinSamples {
		t.Fatalf("expected at least %d samples, got %d", expectedMinSamples, len(samples))
	}

	t.Logf("Decoded %d samples (%.2f seconds)", len(samples), float64(len(samples))/float64(targetSampleRate))
}

func TestDecodeFile_NotFound(t *testing.T) {
	_, err := DecodeFile("nonexistent.m4a")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
