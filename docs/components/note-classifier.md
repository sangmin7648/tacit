---
source: [tacit/core/internal/components/note-classifier]
verified: a6db5d7
---

# note-classifier

Asks an LLM (Ollama or the Claude CLI, chosen by settings) to title and categorise a transcript, or to skip it. See [classification](../concepts/classification.md).

Also the single step that repairs whatever comes back into something [note-manager](note-manager.md) accepts, including the `unsorted` fallback, so a weak answer never costs the transcript.
