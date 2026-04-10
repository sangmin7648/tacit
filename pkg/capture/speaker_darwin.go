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
	"sync"
	"unsafe"
)

// Speaker captures system audio output (all apps) using ScreenCaptureKit.
// Requires macOS 13.0+ and Screen Recording permission.
// The captured audio is NOT affected by the microphone mute state.
type Speaker struct {
	cap *C.SpeakerCapture
}

// NewSpeaker creates a new system-audio capture instance.
// The ScreenCaptureKit stream is started lazily in Stream().
func NewSpeaker() (*Speaker, error) {
	// We create a temporary placeholder handle; the real handle is set in
	// Stream() once we have a channel to deliver samples into.
	// speaker_create is called lazily in Stream so we can pass the channel handle.
	return &Speaker{}, nil
}

// speakerChan wraps the audio channel with a once-guarded close so both the
// context-cancellation path and the unexpected-stop callback can safely signal
// the pipeline without a double-close panic.
type speakerChan struct {
	data   chan []int16
	handle cgo.Handle // points back to itself; used for self-deletion
	once   sync.Once
}

func (sc *speakerChan) closeAndFree() {
	sc.once.Do(func() {
		close(sc.data)
		sc.handle.Delete()
	})
}

// Stream starts system-audio capture and returns a channel of int16 chunks.
// The channel is closed when ctx is cancelled or when SCStream stops unexpectedly.
func (s *Speaker) Stream(ctx context.Context) (<-chan []int16, error) {
	sc := &speakerChan{data: make(chan []int16, 128)}
	// Register sc in the cgo handle registry; the handle value is stored in sc
	// itself so the ObjC callbacks can call closeAndFree via a single pointer.
	sc.handle = cgo.NewHandle(sc)

	var errCStr *C.char
	cap := C.speaker_create(C.uintptr_t(sc.handle), &errCStr)
	if errCStr != nil {
		msg := C.GoString(errCStr)
		C.free(unsafe.Pointer(errCStr))
		sc.closeAndFree()
		return nil, fmt.Errorf("speaker capture: %s", msg)
	}
	if cap == nil {
		sc.closeAndFree()
		return nil, fmt.Errorf("speaker capture: unknown error")
	}

	s.cap = cap

	go func() {
		<-ctx.Done()
		// Set output.stopped = YES before stopping so didStopWithError won't
		// fire the Go callback again after we deliberately stop the stream.
		C.speaker_stop(cap)
		s.cap = nil
		sc.closeAndFree()
	}()

	log.Printf("System audio capture started (ScreenCaptureKit, 16kHz mono)")
	return sc.data, nil
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
	sc, ok := handle.Value().(*speakerChan)
	if !ok {
		return
	}

	n := int(count)
	buf := unsafe.Slice((*int16)(unsafe.Pointer(samples)), n)
	dst := make([]int16, n)
	copy(dst, buf)

	select {
	case sc.data <- dst:
	default:
		// Drop frame if the pipeline is slow — prevents audio thread stall.
	}
}

// tacitSpeakerStoppedCallback is called from Objective-C when SCStream stops
// unexpectedly (not due to an explicit speaker_stop call).  Closing the channel
// unblocks the pipeline's "for chunk := range stream" loop so it can detect
// the outage and restart if desired.
//
//export tacitSpeakerStoppedCallback
func tacitSpeakerStoppedCallback(h C.uintptr_t) {
	handle := cgo.Handle(h)
	sc, ok := handle.Value().(*speakerChan)
	if !ok {
		return
	}
	log.Printf("System audio capture: SCStream stopped unexpectedly, closing stream channel")
	sc.closeAndFree()
}
