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
	isActive    bool
}

// NewSegmentBuffer creates a buffer with the given sample rate and minimum duration.
func NewSegmentBuffer(sampleRate int, minDuration time.Duration) *SegmentBuffer {
	return &SegmentBuffer{
		sampleRate:  sampleRate,
		minDuration: minDuration,
	}
}

// Start marks the beginning of a speech segment and records the start time.
func (b *SegmentBuffer) Start() {
	b.isActive = true
	b.startTime = time.Now()
	b.samples = b.samples[:0]
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
	if b.sampleRate == 0 {
		return 0
	}
	return time.Duration(float64(len(b.samples)) / float64(b.sampleRate) * float64(time.Second))
}
