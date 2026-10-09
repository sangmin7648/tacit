---
source: [tacit/pkg/daemon]
verified: f0db000
---

# daemon

PID file handling for the `listen` process: write, read, remove, and detect a stale file whose process is gone; plus status and stop (and stop-and-wait), shared by the CLI and the app so they cannot disagree. This file is how `status`, `stop`, the installer and the app all find out whether a daemon is running. See [daemon-and-app](../concepts/daemon-and-app.md).
