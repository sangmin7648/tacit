# sttdb Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-28

## Active Technologies

- Go (latest stable, 1.23+) + malgo (audio capture), ten-vad (VAD), whisper.cpp (STT), Claude Code CLI (post-processing) (001-stt-knowledge-db)

## Project Structure

```text
cmd/sttdb/     — CLI entry point
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

## Workflow

- 코드 수정 후 반드시 `make e2e-test`를 실행하여 파이프라인 동작을 확인할 것

## Code Style

Go (latest stable, 1.23+): Follow standard conventions

## Recent Changes

- 001-stt-knowledge-db: Implemented start command with real-time mic capture (malgo) → VAD (ten-vad) → STT (whisper.cpp) → classify (Claude CLI) → store pipeline

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
