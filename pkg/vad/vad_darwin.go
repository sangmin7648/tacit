//go:build darwin

package vad

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/ten-vad/include
#cgo LDFLAGS: -F${SRCDIR}/../../third_party/ten-vad/lib/macOS -framework ten_vad -Wl,-rpath,${SRCDIR}/../../third_party/ten-vad/lib/macOS
#include <ten_vad.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// VAD wraps the ten-vad voice activity detector.
type VAD struct {
	handle C.ten_vad_handle_t
	hop    int
}

// New creates a VAD instance.
// hopSize: number of samples per frame (160 or 256 for 16kHz = 10ms or 16ms).
// threshold: detection threshold [0.0, 1.0].
func New(hopSize int, threshold float32) (*VAD, error) {
	var handle C.ten_vad_handle_t
	ret := C.ten_vad_create(&handle, C.size_t(hopSize), C.float(threshold))
	if ret != 0 {
		return nil, fmt.Errorf("ten_vad_create failed: %d", int(ret))
	}
	return &VAD{handle: handle, hop: hopSize}, nil
}

// Process runs VAD on one frame of int16 audio samples.
// Returns (probability, isSpeech, error).
func (v *VAD) Process(samples []int16) (float32, bool, error) {
	if len(samples) != v.hop {
		return 0, false, fmt.Errorf("expected %d samples, got %d", v.hop, len(samples))
	}

	var prob C.float
	var flag C.int

	ret := C.ten_vad_process(
		v.handle,
		(*C.int16_t)(unsafe.Pointer(&samples[0])),
		C.size_t(len(samples)),
		&prob,
		&flag,
	)
	if ret != 0 {
		return 0, false, fmt.Errorf("ten_vad_process failed: %d", int(ret))
	}

	return float32(prob), flag == 1, nil
}

// HopSize returns the configured hop size.
func (v *VAD) HopSize() int {
	return v.hop
}

// Close releases VAD resources.
func (v *VAD) Close() {
	if v.handle != nil {
		C.ten_vad_destroy(&v.handle)
		v.handle = nil
	}
}
