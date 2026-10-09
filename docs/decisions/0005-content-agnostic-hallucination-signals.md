---
status: accepted
date: 2026-09-07
source: [tacit/core/internal/components/note-classifier, tacit/core/internal/components/transcriber, tacit/core/workflows/listen]
verified: a6db5d7
---

# 0005. Catch hallucinations with content-agnostic signals

**Decision.** Alongside the phrase denylist, detect hallucinations by properties that do not depend on what was said: characters per second of audio, runs of one repeated sentence, and verbatim recurrence of the same transcript within a rolling window.

**Why.** A denylist only catches sentences someone has already seen, and Whisper has an open-ended supply, in every language the user speaks. But hallucinations share a fingerprint real speech lacks: they are sparse for their duration, they loop, and they recur far more often than any genuine remark. The classifier cannot see recurrence across calls; a deterministic gate can.

**Consequences.**
- Every signal is tuned to keep when unsure ([0003](0003-never-drop-a-transcript.md)): the first occurrences in a window always pass, and thresholds are set so only clear cases fire.
- The user can extend the denylist and tune or disable each gate.
- Drops are reported as events with a reason, so a wrongly dropped utterance can be traced. See [hallucination-filtering](../concepts/hallucination-filtering.md).
