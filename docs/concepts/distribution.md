---
source: [install.sh, tacit/Makefile, .github/workflows, tacit/app/desktop/upgrade.go]
verified: a6db5d7
---

# Distribution, install and update

## What ships

A release has one artifact that matters: **Tacit.app**, a zip containing the app and, inside it, the CLI and the VAD framework. `install.sh` unpacks it into `~/Applications` and links `~/.local/bin/tacit` to the bundled CLI. The terminal and the app therefore always run the same build and share one update path. See [0008](../decisions/0008-one-installer-for-cli-and-app.md).

Builds are Apple Silicon only, ad-hoc signed and not notarized. The installer fetches with curl, which sets no quarantine flag, so Gatekeeper does not check it. That is why the README tells users to install through the script rather than a browser download.

## Updating

`tacit update` and the app's own "Update" menu item both rerun `install.sh`. The latest version is read from the GitHub `releases/latest` redirect rather than the REST API, which is rate-limited and breaks behind shared IPs.

The installer refuses to run while the app or a daemon is running, because replacing a running daemon's binary loses its macOS permissions. For the app, a detached helper waits for the app to quit, downloads and runs the installer (a failed download is a failure, never an empty script that "succeeds"), records the result, and reopens the app, which resumes listening if it was listening.

Development builds (anything not a clean release tag) never offer updates.

## Versioning

The release tag is stamped into both binaries and the app's property list at build time.

## Release flow

Pushing a `v*` tag builds on a macOS runner, signs ad hoc, packages, and publishes a GitHub release. CI on every PR builds, tests, and builds the app.
