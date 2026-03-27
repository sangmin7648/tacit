# Implementation Plan: STT 기반 지식 DB 자동 구축

**Branch**: `001-stt-knowledge-db` | **Date**: 2026-03-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-stt-knowledge-db/spec.md`

## Summary

마이크 → VAD → 로컬 Whisper STT → Anthropic API로 제목/요약/카테고리 자동 생성 → 마크다운 지식 파일 저장. CLI로 데몬 제어 및 검색, MCP 서버로 AI 에이전트에게 지식 검색/조회 tool 제공.

핵심 기술 스택: Silero VAD (plandem/silero-go) + whisper.cpp Go bindings + Anthropic Go SDK (Haiku 4.5) + 공식 MCP Go SDK (stdio).

## Technical Context

**Language/Version**: Go (latest stable, 1.23+)
**Primary Dependencies**: plandem/silero-go (VAD+audio), whisper.cpp Go bindings (STT), anthropic-sdk-go (post-processing), modelcontextprotocol/go-sdk (MCP)
**Storage**: Local filesystem (`~/.sttdb/`) — markdown files with YAML frontmatter, hierarchical category directories
**Testing**: `go test ./...` (TDD strict per constitution)
**Target Platform**: macOS (primary), Linux (secondary)
**Project Type**: CLI daemon + MCP server (core library + multiple entry points)
**Performance Goals**: Mic activation <3s, STT save <5s after speech ends, AI agent response <30s with 100+ entries
**Constraints**: Negligible CPU/memory during idle (always-on daemon), STT offline-capable (Whisper local), post-processing requires internet (Anthropic API)
**Scale/Scope**: Single user, 100+ knowledge entries target, single language (Korean)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Check

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. TDD** | PASS | All components (VAD, STT, storage, search, MCP) will follow Red→Green→Refactor. Tests committed with implementation. |
| **II. Simplicity/YAGNI** | PASS | 5 external dependencies, each justified by a core requirement. No plugin system, no feature flags. Phase 2 (desktop app) deferred. |
| **Tech: Go** | PASS | Go latest stable |
| **Tech: VAD→STT pipeline** | PASS | Silero VAD activates Whisper only on speech detection. No continuous STT streaming. |
| **Tech: Local-first storage** | PASS | All knowledge stored as local markdown files in `~/.sttdb/`. No external DB. |
| **Tech: Preprocessing** | PASS | Anthropic API generates title/summary/category before storage. Files are query-ready. |
| **Tech: AI Agent Interface** | PASS | MCP server exposes search/list/get tools via official Go SDK. |
| **Tech: Resource Efficiency** | PASS | Silero VAD: 0.43% CPU during continuous monitoring. Whisper only runs on speech segments. |

### Post-Design Check

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. TDD** | PASS | Data model supports testable interfaces. Storage/search/MCP handlers all testable in isolation. |
| **II. Simplicity/YAGNI** | PASS | Flat data model (1 entity + config). No ORM, no search engine, no database. File glob + string matching for search. |
| **Tech: Preprocessing** | PASS with note | Changed from Claude Code CLI to Anthropic Go SDK — ~100x cheaper ($0.001 vs $0.04-0.15 per entry). Same Claude model, direct API call. |

## Project Structure

### Documentation (this feature)

```text
specs/001-stt-knowledge-db/
├── plan.md              # This file
├── research.md          # Phase 0 output — technology decisions
├── data-model.md        # Phase 1 output — entity definitions
├── quickstart.md        # Phase 1 output — developer setup guide
├── contracts/           # Phase 1 output — MCP tool contracts
│   └── mcp-tools.md     # MCP tool definitions
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
sttdb/
├── cmd/
│   ├── cli/             # CLI entry point (sttdb start/stop/status/search/list)
│   │   └── main.go
│   └── mcp/             # MCP server entry point (stdio transport)
│       └── main.go
├── pkg/
│   ├── audio/           # Audio capture + VAD integration
│   │   ├── capture.go   # Microphone capture via miniaudio
│   │   ├── vad.go       # Silero VAD wrapper
│   │   └── segment.go   # Speech segment buffer management
│   ├── stt/             # Speech-to-text
│   │   ├── whisper.go   # whisper.cpp Go bindings wrapper
│   │   └── model.go     # Model loading and configuration
│   ├── process/         # Post-processing (title/summary/category)
│   │   ├── classify.go  # Anthropic API classification
│   │   └── schema.go    # JSON schema definitions
│   ├── storage/         # Knowledge file management
│   │   ├── writer.go    # Markdown file writer (frontmatter + content)
│   │   ├── reader.go    # Markdown file reader/parser
│   │   ├── search.go    # File glob + keyword matching search
│   │   └── listing.go   # Directory traversal and listing
│   ├── config/          # Configuration management
│   │   └── config.go    # YAML config loading from ~/.sttdb/config.yaml
│   ├── daemon/          # Daemon lifecycle management
│   │   └── pid.go       # PID file management + stale detection
│   └── pipeline/        # Pipeline orchestration
│       └── pipeline.go  # VAD→STT→Process→Store pipeline wiring
├── go.mod
└── go.sum
```

**Structure Decision**: Go idiomatic `cmd/` + `pkg/` layout, matching the spec's "코어 라이브러리 + 다중 진입점" architecture. Each `pkg/` package is independently testable. CLI and MCP are thin entry points calling into `pkg/`.

## Complexity Tracking

> No constitution violations. All design choices are the simplest option that satisfies requirements.
