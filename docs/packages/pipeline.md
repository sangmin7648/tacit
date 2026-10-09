---
source: [tacit/pkg/pipeline]
verified: f0db000
---

# pipeline

Orchestrates capture, VAD, STT, filtering, classification and storage for the daemon. The stage-by-stage description is in [pipeline](../concepts/pipeline.md).

Owns the cross-cutting behaviours: restarting a dead source, batching classification, the never-drop-a-transcript guarantee ([0003](../decisions/0003-never-drop-a-transcript.md)), and emitting [events](../concepts/events.md).

The only package that knows the order of the stages; the others know nothing of each other.
