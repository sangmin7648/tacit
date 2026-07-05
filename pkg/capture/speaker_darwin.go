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
// A single Speaker may be Stream()ed repeatedly (the pipeline restarts sources
// after sleep/wake); each call runs an independent capture session.
type Speaker struct {
	session *speakerSession
}

// NewSpeaker creates a new system-audio capture instance.
// The ScreenCaptureKit stream is started lazily in Stream().
func NewSpeaker() (*Speaker, error) {
	// We create a temporary placeholder handle; the real handle is set in
	// Stream() once we have a channel to deliver samples into.
	// speaker_create is called lazily in Stream so we can pass the channel handle.
	return &Speaker{}, nil
}

// speakerSession owns one ScreenCaptureKit capture session: the data channel,
// the cgo handle the ObjC callbacks use to find it, and the native SCStream
// handle.  Teardown is split into two once-guarded steps so the context-cancel
// path, an explicit Close, and the unexpected-stop callback can all fire without
// double-close panics or double-free of the native stream:
//   - closeChan:  close the data channel + delete the cgo handle
//   - stopNative: release the native SCStream (C.speaker_stop)
type speakerSession struct {
	data      chan []int16
	handle    cgo.Handle
	cap       *C.SpeakerCapture
	closeOnce sync.Once
	stopOnce  sync.Once
}

func (s *speakerSession) closeChan() {
	s.closeOnce.Do(func() {
		close(s.data)
		s.handle.Delete()
	})
}

func (s *speakerSession) stopNative() {
	s.stopOnce.Do(func() {
		if s.cap != nil {
			C.speaker_stop(s.cap)
			s.cap = nil
		}
	})
}

// teardown fully releases the session (native stream + channel).  Idempotent.
func (s *speakerSession) teardown() {
	s.stopNative()
	s.closeChan()
}

// Stream starts system-audio capture and returns a channel of int16 chunks.
// The channel is closed when ctx is cancelled or when SCStream stops unexpectedly.
func (s *Speaker) Stream(ctx context.Context) (<-chan []int16, error) {
	// Release any prior session's native stream before starting a new one so
	// repeated restarts (sleep/wake) don't leak SCStream instances.
	if s.session != nil {
		s.session.teardown()
	}

	sess := &speakerSession{data: make(chan []int16, 128)}
	// Register sess in the cgo handle registry; the handle value is stored in
	// sess itself so the ObjC callbacks can reach it via a single pointer.
	sess.handle = cgo.NewHandle(sess)

	var errCStr *C.char
	cap := C.speaker_create(C.uintptr_t(sess.handle), &errCStr)
	if errCStr != nil {
		msg := C.GoString(errCStr)
		C.free(unsafe.Pointer(errCStr))
		sess.closeChan()
		return nil, fmt.Errorf("speaker capture: %s", msg)
	}
	if cap == nil {
		sess.closeChan()
		return nil, fmt.Errorf("speaker capture: unknown error")
	}

	sess.cap = cap
	s.session = sess

	go func() {
		<-ctx.Done()
		// stopNative sets output.stopped = YES before stopping so
		// didStopWithError won't re-fire the Go callback for our deliberate stop.
		sess.teardown()
	}()

	log.Printf("System audio capture started (ScreenCaptureKit, 16kHz mono)")
	return sess.data, nil
}

// Close stops capture and releases resources if Stream was called.
func (s *Speaker) Close() {
	if s.session != nil {
		s.session.teardown()
	}
}

// tacitSpeakerSamplesCallback is called from Objective-C on each audio chunk.
// It converts the C int16 array into a Go slice and sends it to the channel.
//
//export tacitSpeakerSamplesCallback
func tacitSpeakerSamplesCallback(h C.uintptr_t, samples *C.int16_t, count C.int) {
	handle := cgo.Handle(h)
	sess, ok := handle.Value().(*speakerSession)
	if !ok {
		return
	}

	n := int(count)
	buf := unsafe.Slice((*int16)(unsafe.Pointer(samples)), n)
	dst := make([]int16, n)
	copy(dst, buf)

	select {
	case sess.data <- dst:
	default:
		// Drop frame if the pipeline is slow — prevents audio thread stall.
	}
}

// tacitSpeakerStoppedCallback is called from Objective-C when SCStream stops
// unexpectedly (not due to an explicit speaker_stop call).  Closing the channel
// unblocks the pipeline's capture loop so it can detect the outage and restart.
// It must NOT call stopNative: we're already inside didStopWithError: and the
// native stream is being torn down by macOS; the leaked handle is released when
// the next Stream() starts or Close()/ctx-cancel runs teardown.
//
//export tacitSpeakerStoppedCallback
func tacitSpeakerStoppedCallback(h C.uintptr_t) {
	handle := cgo.Handle(h)
	sess, ok := handle.Value().(*speakerSession)
	if !ok {
		return
	}
	log.Printf("System audio capture: SCStream stopped unexpectedly, closing stream channel")
	sess.closeChan()
}
