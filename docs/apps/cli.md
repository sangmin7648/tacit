---
source: [tacit/app/cli]
verified: f938cbf
---

# app/cli: the `tacit` command

Command groups:

- **Setup and upkeep:** `setup`, `update`, `install-skills`, `config view|edit|set|unset`, `version`. Through [onboard](../workflows/onboard.md) and [configure](../workflows/configure.md). `setup` shows what it found on the Mac and the recommendation for each question, and starts on the user's current answer once set up.
- **Daemon:** `listen` runs [listen](../workflows/listen.md) in the foreground; `stop` and `status` go through [control](../workflows/control.md).
- **Read side:** `list`, `search`, `get`, the commands skills call, through [browse](../workflows/browse.md). They offer machine-readable output so agents need not parse prose.

The CLI only parses arguments and prints; every decision is a workflow's, so the app makes the same ones. It is the one binary that links the audio stack, because only `listen` runs the pipeline.
