# tacit docs

High-level documentation: concepts, package responsibilities, decisions, and domain terms. Code-level detail lives in the code. Rules for keeping these pages current are in [CLAUDE.md](../CLAUDE.md).

Start with [overview](overview.md); look words up in the [glossary](glossary.md).

## Concepts

| Page | Covers | Verified |
|---|---|---|
| [pipeline](concepts/pipeline.md) | Audio to entry, stage by stage; restart and batching | a6db5d7 |
| [segmentation](concepts/segmentation.md) | Segments vs sessions, the two caps, pre-roll | a6db5d7 |
| [hallucination-filtering](concepts/hallucination-filtering.md) | The signals that remove Whisper's invented text | a6db5d7 |
| [classification](concepts/classification.md) | LLM titling, skip, repair, batching | a6db5d7 |
| [knowledge-entry](concepts/knowledge-entry.md) | Entry format, storage rules, what lives in `~/.tacit/` | a6db5d7 |
| [search](concepts/search.md) | List, search, get; why lexical | a6db5d7 |
| [events](concepts/events.md) | Pipeline events as data, transport, rules | a6db5d7 |
| [configuration](concepts/configuration.md) | Reference/override layering, shared setup | a6db5d7 |
| [daemon-and-app](concepts/daemon-and-app.md) | One daemon, terminal and app front ends, ownership | a6db5d7 |
| [distribution](concepts/distribution.md) | Install, update, release | a6db5d7 |
| [skills](concepts/skills.md) | Agent skills and why they are not a server | a6db5d7 |

## Code

The folders mirror `tacit/`. See [0011](decisions/0011-app-workflows-components.md) for the rules.

- **Apps:** [cli](apps/cli.md) · [desktop](apps/desktop.md) · [packaging-mac](apps/packaging-mac.md)
- **Workflows:** [listen](workflows/listen.md) · [control](workflows/control.md) · [onboard](workflows/onboard.md) · [browse](workflows/browse.md) · [configure](workflows/configure.md)
- **Components:** [mic-recorder](components/mic-recorder.md) · [speech-detector](components/speech-detector.md) · [transcriber](components/transcriber.md) · [model-downloader](components/model-downloader.md) · [note-classifier](components/note-classifier.md) · [note-manager](components/note-manager.md) · [setting-manager](components/setting-manager.md) · [skill-installer](components/skill-installer.md) · [status-reporter](components/status-reporter.md)

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
| [0010](decisions/0010-live-audio-only.md) | Process live audio only | accepted |
| [0011](decisions/0011-app-workflows-components.md) | Arrange the code as apps, workflows and components | accepted |
