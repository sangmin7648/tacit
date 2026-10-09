---
source: [tacit/app, tacit/core, tacit/core/internal/components/skill-installer]
verified: a6db5d7
---

# Overview

tacit captures what you say, turns it into structured notes, and lets AI agents search those notes. Everything up to the agent runs on the user's Mac.

```
microphone -> VAD -> segment -> Whisper -> filters -> LLM classify -> Markdown entry
                                                                          |
                                      AI agent  <- skill <- tacit search / list / get
```

## The four deliverables

| Deliverable | What it is | Page |
|---|---|---|
| `tacit` CLI | Setup, the `listen` daemon, and the read commands (`list`, `search`, `get`) | [cli](apps/cli.md) |
| Tacit.app | Menu-bar app that starts and watches the daemon, plus onboarding, settings, and a knowledge browser | [desktop](apps/desktop.md), [packaging-mac](apps/packaging-mac.md) |
| Skills | Instructions installed into an AI agent so it can query the knowledge base | [skills](concepts/skills.md) |
| Knowledge base | The Markdown files under `~/.tacit/` | [knowledge-entry](concepts/knowledge-entry.md) |

## Code layout

Everything is under `tacit/`. `app/` holds the two binaries, `core/workflows/` what they can ask for (listen, control, onboard, browse, configure), and `core/internal/components/` the parts the workflows combine. Dependencies run one way, and the compiler enforces it. See [0011](decisions/0011-app-workflows-components.md).

## Reading order

1. [pipeline](concepts/pipeline.md): how audio becomes an entry.
2. [daemon-and-app](concepts/daemon-and-app.md): who runs the pipeline and how the app and CLI share it.
3. [classification](concepts/classification.md) and [hallucination-filtering](concepts/hallucination-filtering.md): the two judgement steps that decide what is kept.
4. [decisions/](decisions/): why the system looks the way it does.

## Principles that shape the design

- **On-device audio.** Audio never leaves the machine. Only transcript text goes to the classifier, and only if the user picks a hosted one (Claude). See [0001](decisions/0001-on-device-stt.md).
- **Nothing said is silently lost.** Failures store the transcript unclassified rather than drop it. See [0003](decisions/0003-never-drop-a-transcript.md).
- **Plain files are the database.** See [0002](decisions/0002-markdown-files-as-the-database.md).
- **One daemon, many front ends.** The terminal and the app run and observe the same process. See [0004](decisions/0004-app-drives-the-cli-daemon.md).
- **Portable binary.** Everything except the AI agent is linked or bundled in. See [0006](decisions/0006-static-linking.md).
