//go:build !darwin

package capture

import (
	"context"
	"fmt"
)

// Speaker is a no-op stub on non-Darwin platforms.
type Speaker struct{}

// NewSpeaker always returns an error on non-Darwin platforms.
func NewSpeaker() (*Speaker, error) {
	return nil, fmt.Errorf("system audio capture is only supported on macOS")
}

func (s *Speaker) Stream(_ context.Context) (<-chan []int16, error) {
	return nil, fmt.Errorf("system audio capture is only supported on macOS")
}

func (s *Speaker) Close() {}
