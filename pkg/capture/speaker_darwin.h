#pragma once
#include <stdint.h>
#include <stddef.h>

// Opaque handle returned by speaker_create.
typedef struct SpeakerCapture SpeakerCapture;

// tacitSpeakerSamplesCallback is defined in speaker_darwin.go (//export).
// Declared here so the .m file can call it.
extern void tacitSpeakerSamplesCallback(uintptr_t handle, int16_t* samples, int count);

// tacitSpeakerStoppedCallback is called when SCStream stops unexpectedly.
// Defined in speaker_darwin.go (//export).
extern void tacitSpeakerStoppedCallback(uintptr_t handle);

// speaker_create starts ScreenCaptureKit system audio capture (macOS 13+).
// On success returns a non-NULL handle and sets *errMsg to NULL.
// On failure returns NULL and sets *errMsg to a malloc'd error string (caller must free).
// goHandle is passed through to tacitSpeakerSamplesCallback as the first argument.
SpeakerCapture* speaker_create(uintptr_t goHandle, char** errMsg);

// speaker_stop stops the stream and releases resources.
void speaker_stop(SpeakerCapture* cap);
