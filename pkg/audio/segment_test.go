package audio

import (
	"testing"
	"time"
)

const testSampleRate = 16000

// makeSamples creates a slice of n float32 samples with value v.
func makeSamples(n int, v float32) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func TestNewSegmentBuffer(t *testing.T) {
	minDur := 2 * time.Second
	buf := NewSegmentBuffer(testSampleRate, minDur)

	if buf.sampleRate != testSampleRate {
		t.Errorf("expected sampleRate %d, got %d", testSampleRate, buf.sampleRate)
	}
	if buf.minDuration != minDur {
		t.Errorf("expected minDuration %v, got %v", minDur, buf.minDuration)
	}
	if buf.IsActive() {
		t.Error("expected new buffer to be inactive")
	}
	if buf.Duration() != 0 {
		t.Errorf("expected zero duration, got %v", buf.Duration())
	}
}

func TestSegmentBuffer_ShortSpeech(t *testing.T) {
	minDur := 2 * time.Second
	buf := NewSegmentBuffer(testSampleRate, minDur)

	buf.Start()

	// Append 1 second of audio (16000 samples) which is less than minDuration (2s).
	samples := makeSamples(testSampleRate*1, 0.5)
	buf.Append(samples)

	seg, ok := buf.Finish()
	if ok {
		t.Error("expected Finish to return false for short speech")
	}
	if seg != nil {
		t.Error("expected nil segment for short speech")
	}
	if buf.IsActive() {
		t.Error("expected buffer to be inactive after Finish")
	}
}

func TestSegmentBuffer_ValidSpeech(t *testing.T) {
	minDur := 2 * time.Second
	buf := NewSegmentBuffer(testSampleRate, minDur)

	buf.Start()

	// Append 3 seconds of audio (48000 samples) which exceeds minDuration (2s).
	samples := makeSamples(testSampleRate*3, 0.7)
	buf.Append(samples)

	seg, ok := buf.Finish()
	if !ok {
		t.Fatal("expected Finish to return true for valid speech")
	}
	if seg == nil {
		t.Fatal("expected non-nil segment")
	}
	if len(seg.Samples) != testSampleRate*3 {
		t.Errorf("expected %d samples, got %d", testSampleRate*3, len(seg.Samples))
	}
	if seg.StartTime.IsZero() {
		t.Error("expected non-zero start time")
	}

	expectedDur := 3 * time.Second
	if seg.Duration != expectedDur {
		t.Errorf("expected duration %v, got %v", expectedDur, seg.Duration)
	}

	// Buffer should be reset after Finish.
	if buf.IsActive() {
		t.Error("expected buffer to be inactive after Finish")
	}
	if buf.Duration() != 0 {
		t.Error("expected zero duration after Finish")
	}
}

func TestSegmentBuffer_Duration(t *testing.T) {
	buf := NewSegmentBuffer(testSampleRate, time.Second)

	buf.Start()

	// 0.5 seconds = 8000 samples
	buf.Append(makeSamples(8000, 0.1))
	got := buf.Duration()
	expected := 500 * time.Millisecond
	if got != expected {
		t.Errorf("expected duration %v, got %v", expected, got)
	}

	// Add another 0.5 seconds = 8000 more samples, total 1.0 seconds
	buf.Append(makeSamples(8000, 0.2))
	got = buf.Duration()
	expected = time.Second
	if got != expected {
		t.Errorf("expected duration %v, got %v", expected, got)
	}

	// 3 seconds = 48000 samples total, add 32000 more
	buf.Append(makeSamples(32000, 0.3))
	got = buf.Duration()
	expected = 3 * time.Second
	if got != expected {
		t.Errorf("expected duration %v, got %v", expected, got)
	}
}

func TestSegmentBuffer_Reset(t *testing.T) {
	buf := NewSegmentBuffer(testSampleRate, time.Second)

	buf.Start()
	buf.Append(makeSamples(testSampleRate*2, 0.5))

	if !buf.IsActive() {
		t.Error("expected buffer to be active before Reset")
	}
	if buf.Duration() == 0 {
		t.Error("expected non-zero duration before Reset")
	}

	buf.Reset()

	if buf.IsActive() {
		t.Error("expected buffer to be inactive after Reset")
	}
	if buf.Duration() != 0 {
		t.Errorf("expected zero duration after Reset, got %v", buf.Duration())
	}
	if len(buf.samples) != 0 {
		t.Error("expected empty samples after Reset")
	}
	if !buf.startTime.IsZero() {
		t.Error("expected zero start time after Reset")
	}
}

func TestSegmentBuffer_MultipleSegments(t *testing.T) {
	minDur := time.Second
	buf := NewSegmentBuffer(testSampleRate, minDur)

	// First segment: 2 seconds.
	buf.Start()
	buf.Append(makeSamples(testSampleRate*2, 0.3))
	seg1, ok := buf.Finish()
	if !ok {
		t.Fatal("expected first segment to succeed")
	}
	if seg1 == nil {
		t.Fatal("expected non-nil first segment")
	}
	if len(seg1.Samples) != testSampleRate*2 {
		t.Errorf("first segment: expected %d samples, got %d", testSampleRate*2, len(seg1.Samples))
	}
	expectedDur1 := 2 * time.Second
	if seg1.Duration != expectedDur1 {
		t.Errorf("first segment: expected duration %v, got %v", expectedDur1, seg1.Duration)
	}

	// Second segment: 5 seconds.
	buf.Start()
	buf.Append(makeSamples(testSampleRate*5, 0.8))
	seg2, ok := buf.Finish()
	if !ok {
		t.Fatal("expected second segment to succeed")
	}
	if seg2 == nil {
		t.Fatal("expected non-nil second segment")
	}
	if len(seg2.Samples) != testSampleRate*5 {
		t.Errorf("second segment: expected %d samples, got %d", testSampleRate*5, len(seg2.Samples))
	}
	expectedDur2 := 5 * time.Second
	if seg2.Duration != expectedDur2 {
		t.Errorf("second segment: expected duration %v, got %v", expectedDur2, seg2.Duration)
	}

	// The two segments should have different start times (both set via time.Now()).
	if seg1.StartTime.Equal(seg2.StartTime) {
		t.Log("warning: start times are equal; this is possible but unlikely with time.Now()")
	}

	// Verify the sample values are independent (first seg has 0.3, second has 0.8).
	if seg1.Samples[0] != 0.3 {
		t.Errorf("first segment sample value: expected 0.3, got %f", seg1.Samples[0])
	}
	if seg2.Samples[0] != 0.8 {
		t.Errorf("second segment sample value: expected 0.8, got %f", seg2.Samples[0])
	}

	// Buffer should be inactive after final Finish.
	if buf.IsActive() {
		t.Error("expected buffer to be inactive after second Finish")
	}
}
