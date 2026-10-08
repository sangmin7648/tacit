---
source: [pkg/audio]
verified: 9825e8e
---

# audio

Two jobs: decode an audio file into the raw samples the pipeline expects, and buffer live speech into segments.

- **Decoding** uses the operating system's own decoder on macOS so common formats work with nothing installed; other platforms fall back to ffmpeg. Used by `tacit process`.
- **Segment buffer** collects samples between speech start and end, applies the minimum duration (too short is discarded) and the maximum (forces a split). See [segmentation](../concepts/segmentation.md).

Depends on nothing else in the project.
