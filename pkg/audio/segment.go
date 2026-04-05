package audio

import (
	"time"
)

// AudioSegment represents a finalized chunk of speech audio.
type AudioSegment struct {
	Samples   []float32     // 16kHz mono PCM samples
	StartTime time.Time     // Segment start timestamp
	Duration  time.Duration // Duration of the segment
}

// SegmentBuffer manages accumulation of audio samples during speech.
type SegmentBuffer struct {
	samples     []float32
	startTime   time.Time
	sampleRate  int           // typically 16000
	minDuration time.Duration // minimum speech duration to keep
	maxDuration time.Duration // pre-allocation cap; 0 = no pre-alloc
	isActive    bool
}

// NewSegmentBuffer creates a buffer with the given sample rate, minimum
// duration, and optional maximum duration. When maxDuration > 0 the backing
// array is pre-allocated to exactly that size at Start() and reused across
// segments, eliminating append-induced reallocations.
func NewSegmentBuffer(sampleRate int, minDuration, maxDuration time.Duration) *SegmentBuffer {
	return &SegmentBuffer{
		sampleRate:  sampleRate,
		minDuration: minDuration,
		maxDuration: maxDuration,
	}
}

// Start marks the beginning of a speech segment and records the start time.
// If maxDuration was set, the backing array is pre-allocated to that capacity
// (or reused from the previous segment) so no reallocation happens during
// Append calls.
func (b *SegmentBuffer) Start() {
	b.isActive = true
	b.startTime = time.Now()
	if b.maxDuration > 0 {
		maxSamples := int(b.maxDuration.Seconds() * float64(b.sampleRate))
		if cap(b.samples) < maxSamples {
			b.samples = make([]float32, 0, maxSamples)
		} else {
			b.samples = b.samples[:0]
		}
	} else {
		b.samples = b.samples[:0]
	}
}

// Append adds samples to the buffer.
func (b *SegmentBuffer) Append(samples []float32) {
	b.samples = append(b.samples, samples...)
}

// Finish finalizes the segment. It returns (segment, true) if the buffered
// duration is at least minDuration, or (nil, false) if the segment is too short.
// The buffer is reset after calling Finish.
func (b *SegmentBuffer) Finish() (*AudioSegment, bool) {
	dur := b.Duration()

	if dur < b.minDuration {
		b.Reset()
		return nil, false
	}

	seg := &AudioSegment{
		Samples:   make([]float32, len(b.samples)),
		StartTime: b.startTime,
		Duration:  dur,
	}
	copy(seg.Samples, b.samples)

	b.Reset()
	return seg, true
}

// IsActive reports whether the buffer is currently accumulating speech.
func (b *SegmentBuffer) IsActive() bool {
	return b.isActive
}

// Reset clears the buffer and all associated state.
func (b *SegmentBuffer) Reset() {
	b.samples = b.samples[:0]
	b.startTime = time.Time{}
	b.isActive = false
}

// Duration returns the current buffered duration based on the sample count.
func (b *SegmentBuffer) Duration() time.Duration {
	return DurationFromSamples(len(b.samples), b.sampleRate)
}
