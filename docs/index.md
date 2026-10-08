# tacit docs

High-level documentation: concepts, package responsibilities, decisions, and domain terms. Code-level detail lives in the code. Rules for keeping these pages current are in [CLAUDE.md](../CLAUDE.md).

Start with [overview](overview.md); look words up in the [glossary](glossary.md).

## Concepts

| Page | Covers | Verified |
|---|---|---|
| [pipeline](concepts/pipeline.md) | Audio to entry, stage by stage; restart and batching | 9825e8e |
| [segmentation](concepts/segmentation.md) | Segments vs sessions, the two caps, pre-roll | 9825e8e |
| [hallucination-filtering](concepts/hallucination-filtering.md) | The signals that remove Whisper's invented text | 9825e8e |
| [classification](concepts/classification.md) | LLM titling, skip, repair, batching | 9825e8e |
| [knowledge-entry](concepts/knowledge-entry.md) | Entry format, storage rules, what lives in `~/.tacit/` | 9825e8e |
| [search](concepts/search.md) | List, search, get; why lexical | 9825e8e |
| [events](concepts/events.md) | Pipeline events as data, transport, rules | 9825e8e |
| [configuration](concepts/configuration.md) | Reference/override layering, shared setup | 9825e8e |
| [daemon-and-app](concepts/daemon-and-app.md) | One daemon, terminal and app front ends, ownership | 9825e8e |
| [distribution](concepts/distribution.md) | Install, update, release | 9825e8e |
| [skills](concepts/skills.md) | Agent skills and why they are not a server | 9825e8e |

## Packages

[audio](packages/audio.md) · [capture](packages/capture.md) · [vad](packages/vad.md) · [stt](packages/stt.md) · [process](packages/process.md) · [pipeline](packages/pipeline.md) · [storage](packages/storage.md) · [search](packages/search.md) · [config](packages/config.md) · [setup](packages/setup.md) · [model](packages/model.md) · [daemon](packages/daemon.md) · [events](packages/events.md) · [skills](packages/skills.md) · [cmd/tacit](packages/cmd-tacit.md) · [cmd/tacit-app](packages/cmd-tacit-app.md)

## Decisions

| # | Decision | Status |
|---|---|---|
| [0001](decisions/0001-on-device-stt.md) | Transcribe on the device | accepted |
| [0002](decisions/0002-markdown-files-as-the-database.md) | Markdown files are the database | accepted |
| [0003](decisions/0003-never-drop-a-transcript.md) | Never drop a successfully transcribed segment | accepted |
| [0004](decisions/0004-app-drives-the-cli-daemon.md) | The app drives the CLI daemon | accepted |
| [0005](decisions/0005-content-agnostic-hallucination-signals.md) | Catch hallucinations with content-agnostic signals | accepted |
| [0006](decisions/0006-static-linking.md) | Link everything except the AI agent into the build | accepted |
| [0007](decisions/0007-app-adopts-its-own-orphaned-daemon.md) | The app adopts a daemon it started earlier | accepted |
| [0008](decisions/0008-one-installer-for-cli-and-app.md) | One installer for the CLI and the app | accepted |
| [0009](decisions/0009-microphone-only.md) | Capture the microphone only | accepted |
