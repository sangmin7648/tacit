# sttdb Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-28

## Active Technologies

- Go (latest stable, 1.23+) + plandem/silero-go (VAD+audio), whisper.cpp Go bindings (STT), anthropic-sdk-go (post-processing), modelcontextprotocol/go-sdk (MCP) (001-stt-knowledge-db)

## Project Structure

```text
cmd/sttdb/     — CLI entry point
pkg/audio/     — Audio decoding (AudioToolbox on macOS, ffmpeg fallback)
pkg/stt/       — Whisper STT (CGo, vendored whisper.cpp)
pkg/process/   — Claude Code CLI post-processing
pkg/storage/   — Knowledge file read/write
pkg/config/    — Configuration
pkg/model/     — Model auto-download
pkg/daemon/    — PID file management
pkg/pipeline/  — VAD→STT→Process→Store orchestration
third_party/   — Vendored C dependencies (whisper.cpp submodule)
```

## Commands

```bash
make build     # Build whisper.cpp + Go binary
make test      # Run all tests
make clean     # Clean build artifacts
```

## Code Style

Go (latest stable, 1.23+): Follow standard conventions

## Recent Changes

- 001-stt-knowledge-db: Added Go (latest stable, 1.23+) + plandem/silero-go (VAD+audio), whisper.cpp Go bindings (STT), anthropic-sdk-go (post-processing), modelcontextprotocol/go-sdk (MCP)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
