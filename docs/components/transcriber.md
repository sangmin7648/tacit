---
source: [tacit/core/internal/components/transcriber]
verified: a6db5d7
---

# transcriber

Speech to text through whisper.cpp (a submodule in `whisper.cpp/`), and the cleanup of what Whisper invents: filler detection, the hallucination phrase denylist, repeat-run removal, the density gate, and the stock-repeat deduper. See [hallucination-filtering](../concepts/hallucination-filtering.md).

- **Build constraint:** compiling this needs whisper.cpp's libraries, so only `make` targets can build it. Never `go build ./...`.
- **Concurrency:** one instance is not safe to share; [listen](../workflows/listen.md) serialises calls.
- Whisper may return nothing for a window it reads as non-speech. That is a decoding verdict, not an error.
