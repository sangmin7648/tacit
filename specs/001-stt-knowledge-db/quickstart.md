# Quickstart: tacit Development Setup

**Date**: 2026-03-28

## Prerequisites

- **Go** 1.23+ (`brew install go`)
- **C compiler + cmake** (Xcode Command Line Tools: `xcode-select --install` && `brew install cmake`)
- **Claude Code CLI** (`claude`) — for text post-processing. Claude 구독 필요.

That's it. whisper.cpp is vendored as a git submodule and built automatically. Audio decoding uses macOS AudioToolbox (no ffmpeg needed). Whisper model is auto-downloaded on first run.

## Environment Setup

```bash
# Clone and enter project
git clone --recursive <repo-url> tacit
cd tacit

# Verify Claude Code CLI is installed and authenticated
claude --version

# Build (builds whisper.cpp stacit lib + Go binary)
make build

# Run tests
make test
```

## Whisper Model

Models are stored in `~/.tacit/models/` and auto-downloaded on first run.

To pre-download or use a different model:

```bash
mkdir -p ~/.tacit/models

# Optional: Download small model for better Korean accuracy (~466 MB)
curl -L -o ~/.tacit/models/ggml-small.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
```

Then set `whisper_model: small` in `~/.tacit/config.yaml`.

## Configuration

Optional. Create `~/.tacit/config.yaml` to override defaults:

```yaml
# Whisper model size (tiny/base/small/medium/large)
whisper_model: base

# Minimum speech duration to process (shorter segments are discarded)
min_speech_duration: 3s

# Silence duration to mark end of speech
silence_duration: 1.5s

# VAD speech detection threshold (0.0-1.0, higher = stricter)
speech_threshold: 0.5

# Anthropic model for post-processing
anthropic_model: claude-haiku-4-5-20251001
```

## Usage

### CLI Commands

```bash
# Build the CLI
make build

# Start voice capture daemon
./tacit start

# Check daemon status
./tacit status

# Stop daemon
./tacit stop

# Search knowledge
./tacit search "에러 핸들링"

# List knowledge entries
./tacit list
./tacit list --category "개발"
```

### MCP Server

```bash
# Build the MCP server
go build -o tacit-mcp ./cmd/mcp/

# Register with Claude Code (user scope)
claude mcp add --transport stdio tacit --scope user -- ./tacit-mcp

# Or add to .mcp.json for project scope
```

## Project Structure

```
cmd/cli/      — CLI entry point (tacit start/stop/status/search/list)
cmd/mcp/      — MCP server entry point (stdio)
pkg/audio/    — Audio capture + VAD
pkg/stt/      — Whisper STT
pkg/process/  — Claude Code CLI post-processing (os/exec)
pkg/storage/  — Knowledge file read/write/search
pkg/config/   — Configuration
pkg/daemon/   — PID file management
pkg/pipeline/ — VAD→STT→Process→Store orchestration
```

## Development Workflow (TDD)

Per constitution, all code follows strict TDD:

```bash
# 1. Red: Write a failing test
go test ./pkg/storage/ -run TestWriteKnowledgeEntry
# FAIL

# 2. Green: Write minimum code to pass
# ... edit storage/writer.go ...
go test ./pkg/storage/ -run TestWriteKnowledgeEntry
# PASS

# 3. Refactor: Clean up while green
go test ./...
# ALL PASS

# 4. Commit
git add pkg/storage/writer.go pkg/storage/writer_test.go
git commit -m "feat(storage): add knowledge entry writer"
```

## macOS Microphone Permission

On first run, macOS will prompt for microphone access. If denied:
- The program will display a clear error message with instructions
- Grant permission in System Settings > Privacy & Security > Microphone

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `cmake: command not found` | `brew install cmake` |
| `whisper.h not found` | Run `make clean && make build` to rebuild whisper.cpp |
| `claude: command not found` | Install Claude Code CLI: `npm install -g @anthropic-ai/claude-code` and run `claude` to authenticate |
| `Microphone permission denied` | Grant in macOS System Settings > Privacy & Security > Microphone |
| `Stale PID file` | Delete `~/.tacit/tacit.pid` manually, or `tacit stop` handles it automatically |
