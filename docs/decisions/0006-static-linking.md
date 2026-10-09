---
status: accepted
date: 2026-03-28
source: [tacit/Makefile, tacit/core/internal/components/transcriber/whisper.cpp, tacit/core/internal/components/speech-detector/ten-vad]
verified: a6db5d7
---

# 0006. Link everything except the AI agent into the build

**Decision.** whisper.cpp and miniaudio are compiled in, ripgrep is embedded, and audio decoding uses the operating system's own decoder. The only runtime dependency the user supplies is the AI agent (the Claude CLI, or Ollama if chosen). The VAD framework travels beside the binary.

**Why.** Users install a tool, not a toolchain. A dependency to install by hand (a library, ffmpeg, grep) is a support burden and a reason to give up.

**Consequences.**
- The build is a CGo build against vendored sources, so ordinary `go build ./...` cannot work and contributors use `make`.
- The build produces portable artifacts for one platform: Apple Silicon macOS.
- Native libraries are cached between builds, so changing a build setting such as the minimum macOS version needs a one-time clean of that cache.
