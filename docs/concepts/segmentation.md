---
source: [tacit/core/workflows/listen, tacit/core/internal/components/speech-detector, tacit/core/internal/components/setting-manager]
verified: 831cd18
---

# Segmentation

Segmentation decides where one utterance ends and what is sent to Whisper and to the classifier. Four timings and two thresholds control it; the per-field reference is in the README.

## Two levels

Segments are [speech-detector](../components/speech-detector.md)'s; sessions are [listen](../workflows/listen.md)'s.

- **Segment**: the unit sent to Whisper. It starts when VAD hears speech and ends after a configured stretch of silence. A cap forces a split in unbroken speech so a segment never grows without bound.
- **Session**: the unit sent to the classifier. Text from the segments of one stretch of speech is joined and flushed when silence ends the session, or earlier once a session cap passes.

## Why both caps exist

Continuous speech, such as a meeting, never produces the silence that ends a session. Without a session cap it would pile minutes of speech into one entry, and a single classification failure would lose all of it. The session cap flushes periodically so the damage of one failure stays small.

The session cap can only act on text that already exists, and text exists only after a segment ends or is split. If segment splitting is switched off, the session cap becomes the split point instead, so it never silently does nothing.

## Sessions are not notes

A session boundary does not always mean a new note: the classifier can fold a session into the previous note when the topic continues ([classification](classification.md)). The caps therefore bound the damage of one failure without splitting a long talk into many notes.

## Pre-roll

VAD tends to fire a frame or two after speech actually begins, clipping the first word and causing mistranscription. A short window of recent audio (about 192 ms) is always kept and prepended when speech starts.

## Short segments

Segments under the minimum duration are discarded as noise (coughs, clicks).
