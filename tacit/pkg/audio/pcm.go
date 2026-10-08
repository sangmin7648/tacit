package audio

import (
	"math"
	"time"
)

// SampleRate is the canonical audio sample rate used throughout the pipeline (16kHz mono).
const SampleRate = 16000

// Int16ToFloat32 converts int16 PCM samples to float32 [-1.0, 1.0].
func Int16ToFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, s := range in {
		out[i] = float32(s) / float32(math.MaxInt16)
	}
	return out
}

// DurationFromSamples computes the duration of audio given sample count and rate.
func DurationFromSamples(count, sampleRate int) time.Duration {
	if sampleRate == 0 {
		return 0
	}
	return time.Duration(float64(count) / float64(sampleRate) * float64(time.Second))
}
