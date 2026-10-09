---
source: [tacit/app/cli]
verified: a6db5d7
---

# app/cli: the `tacit` command

Command groups:

- **Setup and upkeep:** `setup`, `update`, `install-skills`, `config view|edit|set|unset`, `version`. Through [onboard](../workflows/onboard.md) and [configure](../workflows/configure.md).
- **Daemon:** `listen` runs [listen](../workflows/listen.md) in the foreground; `stop` and `status` go through [control](../workflows/control.md).
- **Read side:** `list`, `search`, `get`, the commands skills call, through [browse](../workflows/browse.md). They offer machine-readable output so agents need not parse prose.

The CLI only parses arguments and prints; every decision is a workflow's, so the app makes the same ones. It is the one binary that links the audio stack, because only `listen` runs the pipeline.
