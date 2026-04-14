# tacit Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-28

## Active Technologies

- Go (latest stable, 1.23+) + malgo (audio capture), ten-vad (VAD), whisper.cpp (STT), Claude Code CLI (post-processing) (001-stt-knowledge-db)

## Project Structure

```text
cmd/tacit/     — CLI entry point
pkg/audio/     — Audio decoding (AudioToolbox on macOS, ffmpeg fallback) + segment buffer
pkg/capture/   — Real-time microphone capture (malgo/miniaudio)
pkg/vad/       — Voice Activity Detection (ten-vad, prebuilt framework)
pkg/stt/       — Whisper STT (CGo, vendored whisper.cpp)
pkg/process/   — Claude Code CLI post-processing
pkg/storage/   — Knowledge file read/write
pkg/config/    — Configuration
pkg/model/     — Model auto-download
pkg/daemon/    — PID file management
pkg/pipeline/  — Capture→VAD→STT→Process→Store orchestration
third_party/   — Vendored C dependencies (whisper.cpp submodule, ten-vad framework)
```

## Commands

```bash
make build          # Build whisper.cpp + Go binary
make test           # Run all tests
make e2e-test   # Build + process test audio file (testdata/test_voice_recording.m4a)
make clean          # Clean build artifacts
```

## Build Verification

- Do NOT verify builds with `go build ./...` — `pkg/stt` references `whisper.h` via CGo and will always fail before `make build`
- Always use `make build` or `make e2e-test` to verify builds
- Pure Go packages unrelated to CGo (e.g. `./skills/`) can be individually verified with `go build ./skills/`

## Build Principles

- All dependencies (whisper.cpp, ten-vad, miniaudio, AudioToolbox, etc.) except the AI agent (Claude Code CLI) must be statically linked/bundled into the binary. The build output must be a portable binary that requires no external library installation at runtime.

## Workflow

- After modifying code, always run `make e2e-test` to verify pipeline behavior

## Code Style

Go (latest stable, 1.23+): Follow standard conventions

## Recent Changes

- 001-stt-knowledge-db: Implemented start command with real-time mic capture (malgo) → VAD (ten-vad) → STT (whisper.cpp) → classify (Claude CLI) → store pipeline

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
