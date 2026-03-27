# Quickstart: sttdb Development Setup

**Date**: 2026-03-28

## Prerequisites

- **Go** 1.23+ (`brew install go`)
- **C compiler** (Xcode Command Line Tools: `xcode-select --install`)
- **ONNX Runtime** shared library (~17 MB) — required by Silero VAD
- **Whisper model** file (downloaded on first run)
- **Anthropic API key** — for text post-processing (title/summary/category generation)

## System Dependencies

### macOS

```bash
# ONNX Runtime (required for Silero VAD)
brew install onnxruntime

# Whisper.cpp (for the shared library)
brew install whisper-cpp

# Or build whisper.cpp from source:
git clone https://github.com/ggml-org/whisper.cpp.git
cd whisper.cpp && make -j
```

### Linux (Ubuntu/Debian)

```bash
# ONNX Runtime
wget https://github.com/microsoft/onnxruntime/releases/download/v1.18.1/onnxruntime-linux-x64-1.18.1.tgz
tar xzf onnxruntime-linux-x64-1.18.1.tgz
sudo cp onnxruntime-linux-x64-1.18.1/lib/* /usr/local/lib/
sudo cp -r onnxruntime-linux-x64-1.18.1/include/* /usr/local/include/
sudo ldconfig
```

## Environment Setup

```bash
# Clone and enter project
git clone <repo-url> sttdb
cd sttdb

# Set Anthropic API key (required for post-processing)
export ANTHROPIC_API_KEY="sk-ant-..."

# Install Go dependencies
go mod download

# Verify build
go build ./...

# Run tests (TDD — must pass on every commit)
go test ./...
```

## Whisper Model Download

Models are stored in `~/.sttdb/models/`. The default `base` model (~142 MB) is downloaded on first run, or manually:

```bash
mkdir -p ~/.sttdb/models
# Download base model (default)
curl -L -o ~/.sttdb/models/ggml-base.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin

# Optional: Download small model for better Korean accuracy (~466 MB)
curl -L -o ~/.sttdb/models/ggml-small.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
```

## Configuration

Optional. Create `~/.sttdb/config.yaml` to override defaults:

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
go build -o sttdb ./cmd/cli/

# Start voice capture daemon
./sttdb start

# Check daemon status
./sttdb status

# Stop daemon
./sttdb stop

# Search knowledge
./sttdb search "에러 핸들링"

# List knowledge entries
./sttdb list
./sttdb list --category "개발"
```

### MCP Server

```bash
# Build the MCP server
go build -o sttdb-mcp ./cmd/mcp/

# Register with Claude Code (user scope)
claude mcp add --transport stdio sttdb --scope user -- ./sttdb-mcp

# Or add to .mcp.json for project scope
```

## Project Structure

```
cmd/cli/      — CLI entry point (sttdb start/stop/status/search/list)
cmd/mcp/      — MCP server entry point (stdio)
pkg/audio/    — Audio capture + VAD
pkg/stt/      — Whisper STT
pkg/process/  — Anthropic API post-processing
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
| `ONNX Runtime not found` | Ensure `brew install onnxruntime` and check `LIBRARY_PATH` |
| `whisper.h not found` | Set `C_INCLUDE_PATH` to whisper.cpp include directory |
| `ANTHROPIC_API_KEY not set` | Export the key or add to shell profile |
| `Microphone permission denied` | Grant in macOS System Settings > Privacy & Security > Microphone |
| `Stale PID file` | Delete `~/.sttdb/sttdb.pid` manually, or `sttdb stop` handles it automatically |
