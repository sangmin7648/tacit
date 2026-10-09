---
source: [tacit/core/workflows/listen, tacit/core/internal/components/mic-recorder, tacit/core/internal/components/speech-detector, tacit/core/internal/components/transcriber]
verified: a6db5d7
---

# Pipeline

The pipeline turns an audio stream into stored entries. It is the core of the daemon. The end-to-end test plays a recorded file through it as if it were the microphone, so it exercises the same stages. See [0010](../decisions/0010-live-audio-only.md).

## Stages

1. **Capture**: the microphone yields 16 kHz mono PCM in small chunks.
2. **Gate**: a cheap energy check rejects near-silent frames before VAD sees them.
3. **VAD**: classifies frames as speech or not, using a confidence threshold.
4. **Segment**: speech frames accumulate into a segment, which ends after enough silence. Segments shorter than the minimum are dropped. See [segmentation](segmentation.md).
5. **Transcribe**: Whisper converts a segment to text. One Whisper instance is shared and calls are serialised.
6. **Filter**: deterministic gates drop filler, hallucinations, and implausibly sparse text. See [hallucination-filtering](hallucination-filtering.md).
7. **Classify**: text from a speech session is queued to a single classify worker, which may batch several queued items into one LLM call. See [classification](classification.md).
8. **Store**: the result is written as an [entry](knowledge-entry.md).

Stages 1 to 6 run per capture source; stage 7 is one shared worker, so batching works across sources.

## Why classification is asynchronous

An LLM call takes seconds and must not stall audio handling. VAD and Whisper keep running while earlier text is classified, and a backlog is drained in batches.

## Staying alive

A source restarts automatically after any capture session ends for a reason other than shutdown: the device vanished, initialisation failed after sleep/wake, or the stream went quiet. A stream that delivers no chunks for a while is treated as dead, because a live stream sends chunks even during silence. This exists because laptops sleep and audio devices disappear without closing their streams.

## Observability

Each stage reports through [events](events.md) in addition to ordinary logs. The logs are for humans debugging the daemon; the events are for programs.

## Invariants

- A transcript that reaches classification is always stored, unless the classifier deliberately skips it. See [0003](../decisions/0003-never-drop-a-transcript.md).
- Concurrent sources share one Whisper instance and one classify worker.
- Memory stays bounded: long speech is split into capped segments as it goes.
