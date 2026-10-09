---
status: accepted
date: 2026-09-27
source: [tacit/app/desktop, tacit/core/internal/components/status-reporter, tacit/core/workflows/control]
verified: a6db5d7
---

# 0004. The app drives the CLI daemon

**Decision.** The Mac app does not run the pipeline in-process. It launches the bundled CLI's `listen` as a separate process and observes it through the PID file and the event log.

**Why.** A daemon started by the app is then identical to one started from a terminal. One way to detect, show and stop it covers both, and `tacit stop` works on either. Running in-process would have made two implementations of "listening" to keep in step.

**Consequences.**
- Events had to be data on disk rather than function calls. See [events](../concepts/events.md).
- A daemon can outlive the app, which needed an ownership rule. See [0007](0007-app-adopts-its-own-orphaned-daemon.md).
- The CLI lives inside the app bundle so the app always launches its own matching version. See [0008](0008-one-installer-for-cli-and-app.md).
