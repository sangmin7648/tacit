---
source: [cmd/tacit]
verified: 9825e8e
---

# cmd/tacit: the CLI

Command groups:

- **Setup and upkeep:** `setup`, `update`, `install-skills`, `config view|edit|set|unset`, `version`.
- **Daemon:** `listen`, `stop`, `status`.
- **One-shot processing:** `process <file>`, used by the end-to-end test.
- **Read side:** `list`, `search`, `get`, the commands skills call. They offer machine-readable output so agents need not parse prose.

The CLI is thin: logic lives in `pkg/`, so the app can reuse it. See [overview](../overview.md).
