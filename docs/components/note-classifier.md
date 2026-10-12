---
source: [tacit/core/internal/components/note-classifier]
verified: 9a2b7a0
---

# note-classifier

Asks an LLM (Ollama or the Claude CLI, chosen by settings) to title and summarise a transcript, or to skip it. See [classification](../concepts/classification.md).

Given the previous note, it also says whether the new text continues it, in which case its title and summary cover both.

Also the single step that repairs whatever comes back into something [note-manager](note-manager.md) accepts, including the title taken from the transcript when the model gives none, so a weak answer never costs the transcript.

Also answers what onboarding needs to know about the machine: whether the Claude CLI is on PATH, and whether Ollama is installed, running and which models it holds. It can pull an Ollama model with progress, so a recommended model that is missing can be fetched from the window.
