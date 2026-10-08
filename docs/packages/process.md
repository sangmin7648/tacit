---
source: [pkg/process]
verified: 9825e8e
---

# process

Everything that happens to text between Whisper and storage.

- **Classifier interface** with two backends (Ollama, Claude CLI) and a factory that selects by configuration. See [classification](../concepts/classification.md).
- **Transcript hygiene**: filler detection, the hallucination phrase denylist, repeat-run removal, the density gate. See [hallucination-filtering](../concepts/hallucination-filtering.md).
- **Stock-repeat deduper**: a rolling-window counter keyed by normalised text, driven by the transcript's own timestamp so it is testable.
- **Finalising a result**: the single step that repairs a classifier answer into something storage accepts, including the `unsorted` fallback.

Pure text in, text out: no audio, no files.
