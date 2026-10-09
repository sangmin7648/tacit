---
source: [tacit/cmd/tacit, tacit/cmd/tacit-app, tacit/pkg/daemon, tacit/pkg/events]
verified: f0db000
---

# Daemon and the Mac app

`tacit listen` is the daemon: a long-running process that runs the [pipeline](pipeline.md). Tacit.app is a menu-bar front end that starts, watches, and stops it.

## One daemon, two ways in

The app does not run the pipeline itself. It launches the CLI bundled inside the app bundle as a child. So a daemon from the app and one from a terminal are the same thing: a PID file and an [event log](events.md). One code path shows and stops either, and `tacit stop` works on both. See [0004](../decisions/0004-app-drives-the-cli-daemon.md).

## How the app knows what is happening

- **Is a daemon running?** Read the PID file, the way `tacit status` does.
- **What is it doing?** Read the event log: the latest event gives the menu-bar glyph.
- **What did it store?** Read the notes folder, as `tacit list` does, and fill the Recent list; a stored event triggers a reload. Building the list from stored events kept deleted notes listed and lost them all when the log rotated.

## Ownership

The app distinguishes a daemon it started from one the user started in a terminal. Quitting the app stops its own; a terminal's is shown and can be stopped from the menu, but is left running on quit.

A daemon outlives an app that crashes or is killed. So the app records the PID of each daemon it starts and, on the next launch, adopts a running daemon that matches. Without that, an orphaned daemon is mistaken for a terminal one and nothing could ever stop it. See [0007](../decisions/0007-app-adopts-its-own-orphaned-daemon.md).

## Windows

Onboarding (first run: classifier choice, model download, microphone permission), settings, and a knowledge browser. Each reuses the CLI's logic ([configuration](configuration.md), [search](search.md)) rather than reimplementing it.

## Permissions

macOS ties the microphone grant to the signed identity of the binary. Replacing a running daemon's binary loses the grant, which is why updates refuse to run while anything is listening. See [distribution](distribution.md).
