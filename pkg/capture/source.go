package capture

import "context"

// AudioSource provides a stream of 16kHz mono int16 PCM audio samples.
// Mic implements it; the pipeline takes the interface so tests can stand in
// for real audio hardware.
type AudioSource interface {
	// Stream starts capturing and returns a channel of sample chunks.
	// The channel is closed when ctx is cancelled.
	Stream(ctx context.Context) (<-chan []int16, error)
	// Close releases all resources.
	Close()
}
