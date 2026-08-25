package stt

/*
#include <whisper.h>
#include <ggml-backend.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

var backendsOnce sync.Once

// Whisper wraps whisper.cpp for speech-to-text transcription.
type Whisper struct {
	ctx *C.struct_whisper_context
}

// NewWhisper loads a whisper model from the given path.
func NewWhisper(modelPath string) (*Whisper, error) {
	// Load ggml backends (Metal, CPU, etc.) once
	backendsOnce.Do(func() {
		C.ggml_backend_load_all()
	})

	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	params := C.whisper_context_default_params()
	ctx := C.whisper_init_from_file_with_params(cPath, params)
	if ctx == nil {
		return nil, fmt.Errorf("failed to load whisper model from %s", modelPath)
	}

	return &Whisper{ctx: ctx}, nil
}

// Options configures a single Transcribe call.
type Options struct {
	// Language is the whisper language code ("auto" to auto-detect, "en", "ko", ...).
	// Empty is treated as "auto".
	Language string
	// InitialPrompt biases decoding toward the given vocabulary ("" = no hint).
	InitialPrompt string
	// Experimental enables non-speech token suppression, which masks symbol
	// tokens such as "(" and "♪". The other decode settings it used to set are
	// whisper.cpp defaults already. Off preserves legacy behavior.
	Experimental bool
}

// Transcribe converts float32 PCM samples (16kHz mono) to text using opts.
func (w *Whisper) Transcribe(ctx context.Context, samples []float32, opts Options) (string, error) {
	if w.ctx == nil {
		return "", fmt.Errorf("whisper context is nil")
	}
	if len(samples) == 0 {
		return "", fmt.Errorf("empty samples")
	}

	params := C.whisper_full_default_params(C.WHISPER_SAMPLING_BEAM_SEARCH)
	params.beam_search.beam_size = 5
	// whisper.cpp sizes the decoder pool from greedy.best_of — not beam_size —
	// as soon as temperature fallback kicks in (see whisper_full_with_state's
	// per-temperature loop). Temperature fallback is on by default, and
	// greedy.best_of defaults to -1 for the beam-search strategy, so leaving it
	// unset collapses decoding to a single greedy decoder exactly on the hard
	// audio that triggered the fallback.
	params.greedy.best_of = 5

	langStr := opts.Language
	if langStr == "" {
		langStr = "auto"
	}
	lang := C.CString(langStr)
	defer C.free(unsafe.Pointer(lang))
	params.language = lang
	params.translate = C.bool(false)
	params.no_timestamps = C.bool(true)
	params.single_segment = C.bool(false)
	params.print_special = C.bool(false)
	params.print_progress = C.bool(false)
	params.print_realtime = C.bool(false)
	params.print_timestamps = C.bool(false)
	params.n_threads = 4

	// Experimental beta channel.
	//
	// The rest of whisper.cpp's anti-hallucination set — no_context, the
	// no_speech / logprob / entropy thresholds, and temperature fallback — is
	// already on in whisper_full_default_params, so re-assigning those values
	// here changed nothing. suppress_nst is the only decode setting this flag
	// actually flips, and it masks symbol tokens ("(", "♪") rather than whole
	// sentences; stock hallucinated phrases made of ordinary words are stripped
	// after transcription instead, in process.FilterHallucinations.
	if opts.Experimental {
		params.suppress_nst = C.bool(true)
	}

	// Set initial prompt if provided
	var cPrompt *C.char
	if opts.InitialPrompt != "" {
		cPrompt = C.CString(opts.InitialPrompt)
		defer C.free(unsafe.Pointer(cPrompt))
		params.initial_prompt = cPrompt
	}

	// Run inference
	ret := C.whisper_full(w.ctx, params, (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	if ret != 0 {
		return "", fmt.Errorf("whisper_full failed with code %d", int(ret))
	}

	// Collect segments
	nSegments := int(C.whisper_full_n_segments(w.ctx))
	var sb strings.Builder
	for i := 0; i < nSegments; i++ {
		text := C.GoString(C.whisper_full_get_segment_text(w.ctx, C.int(i)))
		sb.WriteString(text)
	}

	return strings.TrimSpace(sb.String()), nil
}

// Close frees the whisper context.
func (w *Whisper) Close() {
	if w.ctx != nil {
		C.whisper_free(w.ctx)
		w.ctx = nil
	}
}
