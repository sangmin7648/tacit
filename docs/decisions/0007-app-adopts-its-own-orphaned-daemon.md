---
status: accepted
date: 2026-10-09
source: [tacit/cmd/tacit-app]
verified: 9825e8e
---

# 0007. The app adopts a daemon it started on an earlier run

**Decision.** The app records the PID of every daemon it starts. On launch it adopts a running daemon whose PID matches, so quitting the app stops it.

**Why.** If the app crashed or was killed while its daemon kept running, the next launch took that daemon for a terminal-started one and left it alone. Quit then left it running with nothing to show or stop it. One such orphan ran for two days from a bundle that had since been rebuilt, and lost its permission grant on wake.

**Consequences.**
- A daemon started from a terminal has no record and is still left alone on quit.
- The record is cleared when the daemon stops, but only if it still names that daemon, so a newer daemon keeps its own record.
- Ownership is a property of who started the process, not of what it is. See [daemon-and-app](../concepts/daemon-and-app.md).
