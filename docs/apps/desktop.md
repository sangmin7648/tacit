---
source: [tacit/app/desktop]
verified: f938cbf
---

# app/desktop: the menu-bar app

Menu-bar app with a web-technology front end for its windows. Backend responsibilities:

- **Daemon control:** start (by launching the bundled CLI's `listen`), stop, restart, adopt. See [daemon-and-app](../concepts/daemon-and-app.md).
- **Icon:** one image per state the user must tell apart: not listening (dimmed), listening, hearing speech (animated waveform), working on a note (spinner) and error. The shapes differ, not just the tint, because a menu-bar template image has no colour; an error outranks every other state.
- **Errors:** shown at the top of the menu in words the user can act on, with the error icon. A daemon that exits on its own is explained from the reason it logged (microphone, speech model, or unknown), and a microphone problem offers a button to the right Settings pane. Listening is refused up front when the microphone is denied: macOS then delivers silence rather than an error, so the daemon would run and hear nothing. The same check runs while listening, because the answer to the first prompt arrives after the daemon is already up: a "Don't Allow" then stops the daemon and shows the error. An error clears when listening starts again.
- **State:** the menu is a pure function of the PID file and the event log ([control](../workflows/control.md)) and the newest notes ([browse](../workflows/browse.md)).
- **Windows:** onboarding, settings, knowledge browser (also where a Recent item opens, instead of the system Markdown app), over [onboard](../workflows/onboard.md), [configure](../workflows/configure.md) and [browse](../workflows/browse.md). They share one stylesheet and follow the system light or dark appearance. The browser groups notes by day, highlights what a search matched and is driven from the keyboard; settings show the few that people change and fold the detection tuning into Advanced.
- **Onboarding trigger:** the window opens on a first run, and once more for users set up under an older onboarding revision, so a release that changes the choices can reach them without a window on every update. See [onboard](../workflows/onboard.md). The speech-model step only exists to download, so it is skipped when the model is already on the Mac, and the microphone step shows it is waiting from the moment the system prompt is asked until the answer arrives.
- **Updates:** check for a release and, when the user asks, answer in a dialog (a menu click closes the menu, so a menu-only answer is invisible); hand off to a detached updater. See [distribution](../concepts/distribution.md).
- **Permissions:** reads the microphone permission and requests it without nagging: onboarding asks, and so does starting to listen while the answer is still open, because the daemon would otherwise reach the microphone, and macOS ask, only after loading its model.

It never runs the pipeline, and none of the workflows it imports do, so it links no audio code. Paths coming from a window are accepted only if they name a Markdown file inside the knowledge base.
