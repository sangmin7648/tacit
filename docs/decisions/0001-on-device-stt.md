---
status: accepted
date: 2026-03-28
source: [tacit/core/internal/components/transcriber, tacit/core/internal/components/model-downloader]
verified: a6db5d7
---

# 0001. Transcribe on the device

**Decision.** Speech-to-text runs locally with whisper.cpp. Audio is never sent to a service.

**Why.** The product listens continuously, including to private conversations. A recording that never leaves the machine is the strongest privacy guarantee available and the easiest one to explain. It also removes per-minute cost and network dependence for an always-on tool.

**Consequences.**
- The Whisper model is downloaded once on first use and is large; users trade accuracy for speed by choosing a model size.
- Only transcript text can reach a hosted service, and only if the user chooses the Claude classifier. A local Ollama classifier keeps everything on the machine.
- Whisper's hallucinations on silence become tacit's problem to filter. See [hallucination-filtering](../concepts/hallucination-filtering.md).
