---
source: [tacit/pkg/daemon]
verified: 9825e8e
---

# daemon

PID file handling for the `listen` process: write, read, remove, and detect a stale file whose process is gone. This file is how `status`, `stop`, the installer and the app all find out whether a daemon is running. See [daemon-and-app](../concepts/daemon-and-app.md).
