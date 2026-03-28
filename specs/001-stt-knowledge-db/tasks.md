# Tasks: STT 기반 지식 DB 자동 구축

**Input**: Design documents from `/specs/001-stt-knowledge-db/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/mcp-tools.md, quickstart.md
**Tests**: TDD strict per constitution. Unit tests included with each implementation task. Integration/E2E tests as separate tasks with `//go:build integration` tag.
**Organization**: Tasks grouped by user story. Each story is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1, US2, US3 (maps to spec.md user stories)
- Exact file paths included in all descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, Go module, and directory structure

- [ ] T001 Initialize Go module (`go mod init`) and create project directory structure per plan.md: `cmd/tatic/`, `pkg/audio/`, `pkg/stt/`, `pkg/process/`, `pkg/storage/`, `pkg/config/`, `pkg/daemon/`, `pkg/pipeline/`, `testdata/`
- [ ] T002 [P] Move test audio file to `testdata/test_voice_recording.m4a` and verify it exists

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core configuration used by all packages across all user stories

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 Implement Config struct with defaults, YAML loader, and unit tests in `pkg/config/config.go` and `pkg/config/config_test.go`. Fields: `whisper_model` (default `"base"`), `min_speech_duration` (default `3s`), `silence_duration` (default `1.5s`), `speech_threshold` (default `0.5`), `claude_model` (default `"haiku"`). Load from `~/.tatic/config.yaml`, return defaults if file absent.

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - 음성으로 지식 자동 수집 (Priority: P1) :dart: MVP

**Goal**: Mic → VAD → STT → Claude Code CLI classify → Markdown knowledge file → CLI daemon control

**Independent Test**: Run `tatic start`, speak into mic, verify markdown file created in `~/.tatic/{category}/` with YAML frontmatter (title, category, created_at) and body (summary + STT transcript)

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation. Integration tests use `//go:build integration` tag.**

- [ ] T004 [P] [US1] Write storage write/read round-trip unit tests in `pkg/storage/writer_test.go`: `TestWriteAndRead_RoundTrip` (write entry → read back → compare fields, verify directory structure `{category}/{timestamp}.md`) and `TestWrite_InvalidCategory` (reject 3+ level categories). See plan.md Storage Unit Tests section for reference implementation.
- [ ] T005 [P] [US1] Write STT integration test in `pkg/stt/whisper_integration_test.go`: `TestWhisperTranscribe_RealAudio` — decode `testdata/test_voice_recording.m4a` → transcribe → assert non-empty text. Skip if `TATIC_WHISPER_MODEL` env not set. See plan.md Integration Test: STT section.
- [ ] T006 [P] [US1] Write classification integration test in `pkg/process/classify_integration_test.go`: `TestClassify_RealCLI` — classify sample Korean text → assert title/summary/category non-empty, title <=100 chars, category max 1 slash. Skip if `claude` CLI not found. See plan.md Integration Test: Post-Processing section.
- [ ] T007 [P] [US1] Write E2E pipeline integration test in `pkg/pipeline/pipeline_integration_test.go`: `TestPipeline_AudioFileToKnowledgeEntry` — decode audio → STT → classify → write → read back → verify frontmatter + body + directory structure. Skip if Whisper model or Claude CLI unavailable. See plan.md E2E Test section.

### Implementation for User Story 1

**Parallel Group 1** — Independent foundation components (no cross-dependencies):

- [ ] T008 [P] [US1] Implement audio file decoder in `pkg/audio/decode.go`: `DecodeFile(path string) ([]float32, error)` — decode m4a/wav/mp3/flac to 16kHz mono float32 PCM using miniaudio decoder. Resample if source rate differs. Include unit test with a generated WAV fixture.
- [ ] T009 [P] [US1] Implement KnowledgeEntry struct and markdown writer in `pkg/storage/writer.go`: Define `KnowledgeEntry` struct (Title, Category, CreatedAt, Summary, Content, FilePath). Implement `Write(baseDir string, entry *KnowledgeEntry) (string, error)` — create category directories, generate `{YYYYMMDD}-{HHmmss}.md` filename, write YAML frontmatter + summary + `---` separator + content. Validate: title non-empty/max 100 chars, category max 2 levels, no path traversal.
- [ ] T010 [P] [US1] Implement Whisper model loading in `pkg/stt/model.go`: Model path resolution from config (`~/.tatic/models/ggml-{model}.bin`), existence check, and loading via whisper.cpp Go bindings.
- [ ] T011 [P] [US1] Implement Silero VAD wrapper in `pkg/audio/vad.go`: Wrap `plandem/silero-go` detector. Configure speech threshold, min silence duration, and speech padding from Config. Provide callback-based API for speech segment detection.
- [ ] T012 [P] [US1] Implement speech segment buffer management in `pkg/audio/segment.go`: Define `AudioSegment` struct (`Samples []float32`, `StartTime time.Time`, `Duration time.Duration`). Buffer incoming PCM samples during speech, track duration, enforce minimum speech duration filter (discard segments < `min_speech_duration`). Include unit tests.
- [ ] T013 [P] [US1] Implement PID file management with stale detection in `pkg/daemon/pid.go`: `WritePID(path string)`, `ReadPID(path string) (int, error)`, `IsRunning(pid int) bool` (signal 0 check), `RemovePID(path string)`, stale PID auto-cleanup. PID file at `~/.tatic/tatic.pid`. Include unit tests.

**Parallel Group 2** — Components depending on Group 1:

- [ ] T014 [P] [US1] Implement markdown reader/parser in `pkg/storage/reader.go`: `Read(filePath string) (*KnowledgeEntry, error)` — parse YAML frontmatter (`title`, `category`, `created_at`), split body into summary (before `---`) and content (after `---`). Set `FilePath` field. Include unit tests.
- [ ] T015 [P] [US1] Implement Whisper transcription wrapper in `pkg/stt/whisper.go`: `NewWhisper(modelPath string) (*Whisper, error)`, `Transcribe(ctx context.Context, samples []float32) (string, error)`, `Close()`. Feed float32 PCM directly to whisper.cpp, return Korean text. Configure language=ko.
- [ ] T016 [P] [US1] Implement Claude Code CLI classification via os/exec in `pkg/process/classify.go`: `Classify(ctx context.Context, sttText string, existingCategories []string) (*ClassifyResult, error)`. Invoke `claude -p --output-format json --json-schema <schema> --model <model> --tools "" --no-session-persistence` with STT text on stdin. Parse JSON response for title/summary/category. Pass existing category list in prompt for consistency. Define `ClassifyResult` struct.
- [ ] T017 [P] [US1] Implement microphone capture via miniaudio in `pkg/audio/capture.go`: Start/stop audio capture using `plandem/silero-go` built-in miniaudio. Feed captured PCM frames to VAD detector. Handle macOS microphone permission denial with clear error message (FR-009).

**Pipeline + CLI** — Sequential, depends on all above:

- [ ] T018 [US1] Implement VAD→STT→Process→Store pipeline orchestration in `pkg/pipeline/pipeline.go`: Wire together audio capture → VAD → segment buffering → Whisper STT → Claude CLI classify → storage write. Accept Config. Run as long-running goroutine. On classify failure: log error, skip saving (no data loss panic). List existing categories from storage for classify prompt context.
- [ ] T019 [US1] Implement CLI entry point with start/stop/status daemon commands in `cmd/tatic/main.go`: `tatic start` (run pipeline in foreground, write PID, handle SIGINT/SIGTERM for graceful shutdown), `tatic stop` (read PID, send SIGTERM, remove PID file), `tatic status` (check PID liveness, report running/stopped). Use `pkg/daemon` for PID management, `pkg/pipeline` for pipeline, `pkg/config` for configuration.

**Checkpoint**: User Story 1 complete. Run `tatic start` → speak → verify markdown file in `~/.tatic/`. Run `tatic stop` → verify daemon terminates. Integration tests pass with `go test -tags integration ./...`

---

## Phase 4: User Story 2 - AI 에이전트 지식 활용 (Priority: P2)

**Goal**: MCP server exposes search/list/get tools so AI agents can query the knowledge DB

**Independent Test**: Pre-populate `~/.tatic/` with sample markdown files, register MCP server with Claude Code, ask Claude a question about stored knowledge, verify it references knowledge DB content

### Tests for User Story 2

- [ ] T020 [P] [US2] Write keyword search unit tests in `pkg/storage/search_test.go`: Test keyword matching across title/summary/content (case-insensitive), `잡담/` default exclusion, category filter, `include_chitchat` flag, match context snippet (~100 chars), results sorted by created_at desc. Create temp directory with sample markdown files as fixtures.
- [ ] T021 [P] [US2] Write directory listing unit tests in `pkg/storage/listing_test.go`: Test list all entries, category filter (includes subcategories), date range filter (`YYYY-MM-DD..YYYY-MM-DD`), sorted by created_at desc. Create temp directory with sample fixtures.

### Implementation for User Story 2

- [ ] T022 [P] [US2] Implement keyword search (file glob + string matching) in `pkg/storage/search.go`: `Search(baseDir string, input SearchInput) (*SearchResult, error)`. Walk category directories, read each file via `reader.go`, match query keywords against title+summary+content (case-insensitive substring). Exclude `잡담/` by default unless `IncludeChitchat=true` or `Category="잡담"`. Return entries with `match_context` (~100 chars around first match). Sort by `created_at` desc.
- [ ] T023 [P] [US2] Implement directory traversal listing with filters in `pkg/storage/listing.go`: `List(baseDir string, input ListInput) (*ListResult, error)`. Walk directories, read frontmatter, filter by category prefix and date range. Sort by `created_at` desc. Exclude `잡담/` by default in unfiltered listing.
- [ ] T024 [US2] Implement MCP server with search_knowledge, list_knowledge, get_knowledge tools in `cmd/tatic/main.go` (as `mcp` subcommand): Use `modelcontextprotocol/go-sdk` with stdio transport. Register 3 tools per contracts/mcp-tools.md schemas. `search_knowledge` → `storage.Search()`, `list_knowledge` → `storage.List()`, `get_knowledge` → `storage.Read()`. Validate `get_knowledge` ID against path traversal. Return `isError: true` for missing entries. Knowledge base root: `~/.tatic/`.

**Checkpoint**: MCP server works. Register with `claude mcp add --transport stdio tatic --scope user -- tatic mcp`. Ask Claude about stored knowledge. All 3 tools functional.

---

## Phase 5: User Story 3 - 지식 DB 구조 탐색 (Priority: P3)

**Goal**: CLI commands for searching and browsing the knowledge DB

**Independent Test**: With knowledge entries stored, run `tatic search "에러"` and `tatic list --category 개발` to verify results display correctly

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation.**

- [ ] T025 [P] [US3] Write CLI search command unit tests in `cmd/tatic/main_test.go`: Test argument parsing (`tatic search <query> --category <cat> --include-chitchat`), output formatting (title, category, date, summary per result), error on missing query argument. Use `storage.Search()` with a temp directory fixture.
- [ ] T026 [P] [US3] Write CLI list command unit tests in `cmd/tatic/main_test.go`: Test argument parsing (`tatic list --category <cat> --date-range <range>`), tabular output format, empty results message. Use `storage.List()` with a temp directory fixture.

### Implementation for User Story 3

- [ ] T027 [US3] Add search command to CLI in `cmd/tatic/main.go`: `tatic search <query> [--category <cat>] [--include-chitchat]` — call `storage.Search()`, display results with title, category, date, and summary. Format output for terminal readability.
- [ ] T028 [US3] Add list command with category filter to CLI in `cmd/tatic/main.go`: `tatic list [--category <cat>] [--date-range <start>..<end>]` — call `storage.List()`, display tabular list of entries with title, category, date.

**Checkpoint**: All CLI commands work: `start`, `stop`, `status`, `search`, `list`. All user stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, robustness, and validation

- [ ] T029 [P] Review and verify edge case handling across packages: minimum speech duration filter enforcement in `pkg/audio/segment.go`, microphone permission error in `pkg/audio/capture.go`, Claude CLI failure graceful skip in `pkg/pipeline/pipeline.go`, stale PID cleanup in `pkg/daemon/pid.go`, path traversal rejection in MCP `get_knowledge`
- [ ] T030 Run quickstart.md validation: verify complete setup from scratch (go mod download, go build, go test, whisper model download, `tatic start/stop/status`, MCP server registration)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (go.mod must exist)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (Config needed by all packages)
- **User Story 2 (Phase 4)**: Depends on Phase 2 + storage reader from US1 (T014)
- **User Story 3 (Phase 5)**: Depends on Phase 4 (search/listing implementations)
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Foundational only — no dependencies on other stories
- **US2 (P2)**: Requires `pkg/storage/reader.go` from US1 (T014) and `pkg/storage/writer.go` (T009) for KnowledgeEntry struct
- **US3 (P3)**: Requires `pkg/storage/search.go` (T022) and `pkg/storage/listing.go` (T023) from US2

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD)
- Parallel Group 1 tasks before Parallel Group 2 tasks
- Models/structs before services using them
- Services before entry points (CLI/MCP)
- Pipeline wiring after all component packages

### Parallel Opportunities

**Phase 3 (US1)**:
- Tests T004-T007: all 4 parallel
- Group 1 (T008-T013): all 6 parallel — independent packages
- Group 2 (T014-T017): all 4 parallel — depend on Group 1 but not on each other
- T018 → T019: sequential (pipeline before CLI)

**Phase 4 (US2)**:
- Tests T020-T021: parallel
- Implementation T022-T023: parallel (different files)
- T024: sequential (MCP server depends on search + listing)

**Phase 5 (US3)**:
- Tests T025-T026: parallel
- T027-T028: sequential (both modify cmd/tatic/main.go)

---

## Parallel Example: User Story 1

```
# Group 1 — Launch all independent components together:
Task T008: "Implement audio decoder in pkg/audio/decode.go"
Task T009: "Implement KnowledgeEntry + writer in pkg/storage/writer.go"
Task T010: "Implement Whisper model loading in pkg/stt/model.go"
Task T011: "Implement VAD wrapper in pkg/audio/vad.go"
Task T012: "Implement segment buffer in pkg/audio/segment.go"
Task T013: "Implement PID management in pkg/daemon/pid.go"

# Group 2 — After Group 1 completes, launch next wave:
Task T014: "Implement markdown reader in pkg/storage/reader.go"
Task T015: "Implement Whisper transcription in pkg/stt/whisper.go"
Task T016: "Implement Claude CLI classification in pkg/process/classify.go"
Task T017: "Implement microphone capture in pkg/audio/capture.go"

# Sequential — Pipeline then CLI:
Task T018: "Implement pipeline in pkg/pipeline/pipeline.go"
Task T019: "Implement CLI in cmd/tatic/main.go"
```

---

## Parallel Example: User Story 2

```
# Tests — Launch together:
Task T020: "Write search tests in pkg/storage/search_test.go"
Task T021: "Write listing tests in pkg/storage/listing_test.go"

# Implementation — Launch together:
Task T022: "Implement search in pkg/storage/search.go"
Task T023: "Implement listing in pkg/storage/listing.go"

# Sequential — MCP server after search + listing:
Task T024: "Implement MCP server as 'mcp' subcommand in cmd/tatic/main.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003)
3. Complete Phase 3: User Story 1 (T004-T019)
4. **STOP and VALIDATE**: `tatic start` → speak → verify markdown file created → `tatic stop`
5. Run integration tests: `TATIC_WHISPER_MODEL=~/.tatic/models/ggml-base.bin go test -tags integration -v ./...`

### Incremental Delivery

1. Setup + Foundational → Go project builds and tests pass
2. User Story 1 → Voice capture → knowledge file (MVP!)
3. User Story 2 → AI agents can query knowledge DB via MCP
4. User Story 3 → Users can search/browse via CLI
5. Each story adds value without breaking previous stories

### Notes

- [P] tasks = different files, no cross-dependencies within parallel group
- Each implementation task follows TDD: write test → fail → implement → pass → commit
- Integration tests require: Whisper model file (`TATIC_WHISPER_MODEL` env) + Claude Code CLI (`claude`)
- Unit tests run with `go test ./...` (no external deps)
- Integration tests run with `go test -tags integration -v -timeout 120s ./...`
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
