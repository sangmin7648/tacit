---
status: accepted
date: 2026-10-09
source: [install.sh, cmd/tacit-app/upgrade.go]
verified: 9825e8e
---

# 0008. One installer for the CLI and the app

**Decision.** `install.sh` installs Tacit.app and links the terminal command to the CLI inside it. `tacit update` and the app's update menu both rerun that script.

**Why.** Two separate installs could drift apart: the app running one version and the terminal another, each with its own update path and its own macOS permissions. One artifact, one script and one update path means the terminal and the app always run the same build.

**Consequences.**
- The installer refuses to run while the app or a daemon is running, because replacing a running daemon's binary costs it its macOS permissions.
- An update resets the microphone grant, since an ad-hoc signed build cannot carry one over.
- An existing CLI-only install is migrated: the old binary becomes the link.
- Builds are not notarized, so installation goes through the script, never a browser download. See [distribution](../concepts/distribution.md).
