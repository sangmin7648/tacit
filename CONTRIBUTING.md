# Contributing

Issues and pull requests are welcome. Please open an issue first for significant changes. How the system fits together, and why, is in [`docs/`](docs/index.md).

## Build from source

Requirements: Go 1.23+, CMake, macOS.

```bash
git clone --recursive https://github.com/sangmin7648/tacit.git
cd tacit/tacit
make build
make install   # installs to ~/.local/bin/tacit
```

If `~/.local/bin` is not in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Test

```bash
make test       # unit tests
make e2e-test   # build + play a test recording through the full pipeline
```

Run `make e2e-test` after changing pipeline behaviour.

> Do not run `go build ./...` directly. The transcriber uses CGo against whisper.cpp and fails until `make build` has produced the libraries.

## Settings reference

Settings live in `~/.tacit/`. Put your own values in `config-override.yaml`; `tacit config edit` opens it, and `tacit config view` shows the result with each field tagged `[default]` or `[override]`. `config.yaml` is a generated reference. Don't edit it.

**Transcription**

| Field | Default | What it does |
|---|---|---|
| `whisper_model` | `large-v3-turbo` | Speech model, downloaded on first use: `base`, `small`, `medium`, `large-v3`, `large-v3-turbo`, … |
| `language` | `auto` | Language code (`en`, `ko`, …). Pinning it reduces wrong-language transcriptions. |
| `initial_prompt` | empty | Names, jargon, and acronyms to help recognition. |
| `transcript_denylist` | empty | Extra phrases to discard, on top of the built-in list of phrases speech models invent over silence. |
| `dedup_window` | `3h` | Drops text that keeps repeating word for word, which is usually a transcription glitch rather than speech. `0` turns it off. |
| `min_char_rate` | `0.2` | Drops results with too little text for the amount of audio. `0` turns it off. |

**When a note starts and ends**

| Field | Default | What it does |
|---|---|---|
| `min_speech_duration` | `2s` | Shorter speech is ignored. |
| `silence_duration` | `10s` | How long a pause ends a note. |
| `max_segment_duration` | `30s` | Longest stretch transcribed at once. `0` removes the limit. |
| `max_session_duration` | `5m` | Longest stretch collected into one note, so a long meeting doesn't become one giant note. `0` removes the limit. |
| `speech_threshold` | `0.5` | How sure tacit must be that it hears speech (0–1). Higher is stricter. |
| `energy_threshold` | `200` | Sounds quieter than this are ignored. |

**Note organizer**

| Field | Default | What it does |
|---|---|---|
| `llm_provider` | `ollama` | `ollama` or `claude`. |
| `llm_model` | `qwen3.5` | Model for organizing notes. For `claude`, a Claude Code model name such as `haiku`. |
| `skill_agent` | `claude` | The AI agent that gets the `/tacit.*` commands. |

Anything you say is kept. If the organizer fails or returns only part of an answer, the note is still saved, with a title taken from your first words and the category `unsorted`. The one exception is speech the organizer judges to be nothing worth keeping, such as filler sounds or "hello, can you hear me?" call chatter.
