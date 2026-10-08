---
source: [pkg/vad]
verified: 9825e8e
---

# vad

Voice activity detection: given one frame of audio, report speech or not with a confidence score. A thin wrapper over the prebuilt ten-vad framework, macOS only.

The framework is shipped beside the binary rather than linked into it, so the release and the app bundle must carry it with the CLI. See [distribution](../concepts/distribution.md).
