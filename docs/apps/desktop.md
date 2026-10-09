---
source: [tacit/app/desktop]
verified: a6db5d7
---

# app/desktop: the menu-bar app

Menu-bar app with a web-technology front end for its windows. Backend responsibilities:

- **Daemon control:** start (by launching the bundled CLI's `listen`), stop, restart, adopt. See [daemon-and-app](../concepts/daemon-and-app.md).
- **State:** the menu is a pure function of the PID file and the event log ([control](../workflows/control.md)) and the newest notes ([browse](../workflows/browse.md)).
- **Windows:** onboarding, settings, knowledge browser, over [onboard](../workflows/onboard.md), [configure](../workflows/configure.md) and [browse](../workflows/browse.md).
- **Updates:** check for a release, hand off to a detached updater. See [distribution](../concepts/distribution.md).
- **Permissions:** reads and requests the microphone permission without nagging.

It never runs the pipeline, and none of the workflows it imports do, so it links no audio code. Paths coming from a window are accepted only if they name a Markdown file inside the knowledge base.
