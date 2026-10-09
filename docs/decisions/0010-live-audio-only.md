---
status: accepted
date: 2026-10-09
source: [tacit/pkg/pipeline, tacit/pkg/audio, tacit/cmd/tacit]
verified: f0db000
---

# 0010. Process live audio only

**Decision.** tacit turns only microphone audio into notes. `tacit process <file>` and the second path through the pipeline that served it were removed. The end-to-end test instead plays an uncompressed WAV recording through a stand-in microphone.

**Why.** The file path skipped VAD and segmentation, so the end-to-end test never exercised what users run, and each run wrote a note into the real knowledge base. No user-facing feature needed it: the app never offered it. Keeping the recording uncompressed means the test reads it without a decoder.

**Consequences.**
- No audio decoder ships. The part of [0006](0006-static-linking.md) about the operating system's decoder, and the ffmpeg fallback on other platforms, no longer apply.
- The end-to-end test writes to a temporary directory, never to `~/.tacit`.
- Importing recordings would need a decoder again, and a decision to match.
