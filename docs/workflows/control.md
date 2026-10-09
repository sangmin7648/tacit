---
source: [tacit/core/workflows/control]
verified: a6db5d7
---

# control

Finds the running daemon, stops it, and follows what it is doing, through [status-reporter](../components/status-reporter.md). Used by `tacit stop`/`status` and by the app.

It exists apart from [listen](listen.md) so the app can watch and stop the daemon without linking the audio stack it never runs. See [0011](../decisions/0011-app-workflows-components.md).
