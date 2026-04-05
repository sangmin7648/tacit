//go:build darwin

package capture

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework ScreenCaptureKit -framework CoreMedia -framework AudioToolbox

#include "speaker_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"runtime/cgo"
	"sync/atomic"
	"unsafe"
)

// Speaker captures system audio output (all apps) using ScreenCaptureKit.
// Requires macOS 13.0+ and Screen Recording permission.
// The captured audio is NOT affected by the microphone mute state.
type Speaker struct {
	cap *C.SpeakerCapture
}

// NewSpeaker creates a new system-audio capture instance and starts the
// ScreenCaptureKit stream.  Returns an error if macOS < 13, permission is
// denied, or the stream cannot be initialised.
func NewSpeaker() (*Speaker, error) {
	// We create a temporary placeholder handle; the real handle is set in
	// Stream() once we have a channel to deliver samples into.
	// speaker_create is called lazily in Stream so we can pass the channel handle.
	return &Speaker{}, nil
}

// Stream starts system-audio capture and returns a channel of int16 chunks.
// The channel is closed when ctx is cancelled.
func (s *Speaker) Stream(ctx context.Context) (<-chan []int16, error) {
	ch := make(chan []int16, 128)
	var stopped atomic.Bool

	// Register the channel in the cgo handle registry.
	handle := cgo.NewHandle(ch)

	var errCStr *C.char
	cap := C.speaker_create(C.uintptr_t(handle), &errCStr)
	if errCStr != nil {
		msg := C.GoString(errCStr)
		C.free(unsafe.Pointer(errCStr))
		handle.Delete()
		close(ch)
		return nil, fmt.Errorf("speaker capture: %s", msg)
	}
	if cap == nil {
		handle.Delete()
		close(ch)
		return nil, fmt.Errorf("speaker capture: unknown error")
	}

	s.cap = cap

	go func() {
		<-ctx.Done()
		stopped.Store(true)
		C.speaker_stop(cap)
		s.cap = nil
		handle.Delete()
		close(ch)
	}()

	log.Printf("System audio capture started (ScreenCaptureKit, 16kHz mono)")
	return ch, nil
}

// Close stops capture and releases resources if Stream was called.
func (s *Speaker) Close() {
	if s.cap != nil {
		C.speaker_stop(s.cap)
		s.cap = nil
	}
}

// tacitSpeakerSamplesCallback is called from Objective-C on each audio chunk.
// It converts the C int16 array into a Go slice and sends it to the channel.
//
//export tacitSpeakerSamplesCallback
func tacitSpeakerSamplesCallback(h C.uintptr_t, samples *C.int16_t, count C.int) {
	handle := cgo.Handle(h)
	ch, ok := handle.Value().(chan []int16)
	if !ok {
		return
	}

	n := int(count)
	buf := unsafe.Slice((*int16)(unsafe.Pointer(samples)), n)
	dst := make([]int16, n)
	copy(dst, buf)

	select {
	case ch <- dst:
	default:
		// Drop frame if the pipeline is slow — prevents audio thread stall.
	}
}
