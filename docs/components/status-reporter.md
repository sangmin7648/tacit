---
source: [tacit/core/internal/components/status-reporter]
verified: a6db5d7
---

# status-reporter

The daemon's two status files: the PID file (write, read, remove, detect a stale one, stop the process it names) and the event log (the vocabulary, a rotating writer, a follower that tails it). [listen](../workflows/listen.md) writes both; [control](../workflows/control.md) reads them. See [events](../concepts/events.md) and [daemon-and-app](../concepts/daemon-and-app.md).
