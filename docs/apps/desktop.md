---
source: [tacit/app/desktop]
verified: 56b7886
---

# app/desktop: the menu-bar app

Menu-bar app with a web-technology front end for its windows. Backend responsibilities:

- **Daemon control:** start (by launching the bundled CLI's `listen`), stop, restart, adopt. See [daemon-and-app](../concepts/daemon-and-app.md).
- **Icon:** one image per state the user must tell apart: not listening (dimmed), listening, hearing speech (animated waveform), working on a note (spinner) and error. The shapes differ, not just the tint, because a menu-bar template image has no colour; an error outranks every other state.
- **State:** the menu is a pure function of the PID file and the event log ([control](../workflows/control.md)) and the newest notes ([browse](../workflows/browse.md)).
- **Windows:** onboarding, settings, knowledge browser (also where a Recent item opens, instead of the system Markdown app), over [onboard](../workflows/onboard.md), [configure](../workflows/configure.md) and [browse](../workflows/browse.md).
- **Updates:** check for a release and, when the user asks, answer in a dialog (a menu click closes the menu, so a menu-only answer is invisible); hand off to a detached updater. See [distribution](../concepts/distribution.md).
- **Permissions:** reads the microphone permission and requests it without nagging: onboarding asks, and so does starting to listen while the answer is still open, because the daemon would otherwise reach the microphone, and macOS ask, only after loading its model.

It never runs the pipeline, and none of the workflows it imports do, so it links no audio code. Paths coming from a window are accepted only if they name a Markdown file inside the knowledge base.
