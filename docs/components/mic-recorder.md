---
source: [tacit/core/internal/components/mic-recorder]
verified: a6db5d7
---

# mic-recorder

Turns the microphone into a stream of 16 kHz mono samples, behind an `AudioSource` interface, and defines that format for every later stage.

The interface exists so the pipeline can be tested with a stand-in microphone. The microphone is the only real implementation; system audio was removed ([0009](../decisions/0009-microphone-only.md)).

The stream closes when its context is cancelled. A stream that goes silent without closing is [listen](../workflows/listen.md)'s problem to detect.
