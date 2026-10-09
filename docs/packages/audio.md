---
source: [tacit/pkg/audio]
verified: f0db000
---

# audio

The audio format the pipeline works in, and the segment buffer: it collects samples between speech start and end, applies the minimum duration (too short is discarded) and the maximum (forces a split). See [segmentation](../concepts/segmentation.md).

It decodes no files: the app processes only live audio. See [0010](../decisions/0010-live-audio-only.md).

Depends on nothing else in the project.
