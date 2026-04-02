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

- `go build ./...` 로 빌드 검증하지 말 것 — `pkg/stt`가 CGo로 whisper.h를 참조하므로 `make build` 전에는 항상 실패한다
- 빌드 검증은 반드시 `make build` 또는 `make e2e-test` 사용
- CGo와 무관한 순수 Go 패키지(예: `./skills/`)만 `go build ./skills/` 로 개별 확인 가능

## Build Principles

- AI agent(Claude Code CLI)를 제외한 모든 의존성(whisper.cpp, ten-vad, miniaudio, AudioToolbox 등)은 바이너리에 정적 링크/번들되어야 한다. 빌드 결과물은 반드시 포터블 바이너리여야 하며, 런타임에 외부 라이브러리 설치를 요구해서는 안 된다.

## Workflow

- 코드 수정 후 반드시 `make e2e-test`를 실행하여 파이프라인 동작을 확인할 것

## Code Style

Go (latest stable, 1.23+): Follow standard conventions

## Recent Changes

- 001-stt-knowledge-db: Implemented start command with real-time mic capture (malgo) → VAD (ten-vad) → STT (whisper.cpp) → classify (Claude CLI) → store pipeline

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
