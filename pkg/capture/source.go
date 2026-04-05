package capture

import "context"

// AudioSource provides a stream of 16kHz mono int16 PCM audio samples.
// Both Mic and Speaker implement this interface.
type AudioSource interface {
	// Stream starts capturing and returns a channel of sample chunks.
	// The channel is closed when ctx is cancelled.
	Stream(ctx context.Context) (<-chan []int16, error)
	// Close releases all resources.
	Close()
}
