---
source: [tacit/pkg/capture]
verified: 9825e8e
---

# capture

Turns a microphone into a stream of 16 kHz mono samples, behind an `AudioSource` interface.

The interface exists so the pipeline can be tested with fake audio and so a source can be replaced without touching it. Microphone is the only implementation; system audio was removed ([0009](../decisions/0009-microphone-only.md)).

The stream closes when its context is cancelled. A stream that goes silent without closing is the pipeline's problem to detect, not this package's. See [pipeline](../concepts/pipeline.md).
