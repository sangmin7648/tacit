---
source: [tacit/core/internal/components/speech-detector]
verified: a6db5d7
---

# speech-detector

Finds speech in a sample stream and reports it as segments: speech started, a long segment split, speech ended. Owns voice activity detection (the prebuilt TEN VAD framework, bundled in `ten-vad/`), the energy gate, pre-roll, and the minimum and maximum segment lengths. See [segmentation](../concepts/segmentation.md).

The VAD framework is shipped beside the binary rather than linked into it, so the release and the app bundle must carry it. macOS only. See [distribution](../concepts/distribution.md).
