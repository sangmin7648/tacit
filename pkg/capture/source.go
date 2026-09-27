package capture

import (
	"context"
	"errors"
)

// ErrPermissionDenied is returned, wrapped, by Speaker.Stream when macOS
// refuses system audio capture because Screen Recording is not granted.
// Retrying does not help: a grant reaches a process only once it restarts,
// and each attempt can put the permission dialog up again.
var ErrPermissionDenied = errors.New("Screen Recording permission not granted")

// AudioSource provides a stream of 16kHz mono int16 PCM audio samples.
// Both Mic and Speaker implement this interface.
type AudioSource interface {
	// Stream starts capturing and returns a channel of sample chunks.
	// The channel is closed when ctx is cancelled.
	Stream(ctx context.Context) (<-chan []int16, error)
	// Close releases all resources.
	Close()
}
