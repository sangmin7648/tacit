---
source: [tacit/pkg/stt]
verified: 9825e8e
---

# stt

Speech-to-text through the vendored whisper.cpp, called via CGo. Loads one model and transcribes sample buffers with per-call options (language, prompt, and the experimental decoding switches).

- **Build constraint:** compiling this package needs whisper.cpp's libraries, so only `make` targets can build it. Never `go build ./...`.
- **Concurrency:** one instance is not safe to share; the pipeline serialises calls. See [pipeline](../concepts/pipeline.md).
- Whisper may return nothing for a window it reads as non-speech. That is a decoding verdict, not an error.
