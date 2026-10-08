---
source: [tacit/skills, tacit/cmd/tacit]
verified: 9825e8e
---

# Skills

A skill is a Markdown instruction file installed into an AI agent. tacit ships two, embedded in the binary and copied into the agent's skills directory by setup or `tacit install-skills`.

| Skill | Purpose |
|---|---|
| `tacit.knowledge` | Answer questions from the knowledge base: runs list and search in parallel, reads promising entries, and summarises |
| `tacit.memorize` | Summarise the current agent conversation and save it as an entry in the same format as spoken ones |

## Why skills, not a server

The agent already runs shell commands, and the knowledge base is plain files, so the skills only teach it the three read commands ([search](search.md)). No process to keep running or protocol to version.

## Agent support

Only Claude is supported today; the configured agent name selects the install directory, so another agent means adding a directory mapping.

## Keeping skills correct

Skill text is user-facing product behaviour: it is how an agent decides when to search and what to run. Changing a CLI command or the entry format means updating the skills in the same change.
