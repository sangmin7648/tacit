---
source: [cmd/tacit-app]
verified: 9825e8e
---

# cmd/tacit-app: the Mac app

Menu-bar app with a web-technology front end for its windows. Backend responsibilities:

- **Daemon control:** start, stop, restart, adopt. See [daemon-and-app](../concepts/daemon-and-app.md).
- **State:** the menu is a pure function of the PID file and the event log.
- **Windows:** onboarding, settings, knowledge browser. Each is a thin service over `pkg/` logic.
- **Updates:** check for a release, hand off to a detached updater. See [distribution](../concepts/distribution.md).
- **Permissions:** reads and requests the microphone permission without nagging.

Paths coming from a window are accepted only if they name a Markdown file inside the knowledge base.
