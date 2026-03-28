# Implementation Plan: STT 기반 지식 DB 자동 구축

**Branch**: `001-stt-knowledge-db` | **Date**: 2026-03-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-stt-knowledge-db/spec.md`

## Summary

마이크 → VAD → 로컬 Whisper STT → Claude Code CLI로 제목/요약/카테고리 자동 생성 → 마크다운 지식 파일 저장. CLI로 데몬 제어 및 검색, MCP 서버로 AI 에이전트에게 지식 검색/조회 tool 제공.

핵심 기술 스택: Silero VAD (plandem/silero-go) + whisper.cpp Go bindings + Claude Code CLI (`os/exec`, 구독 포함 $0) + 공식 MCP Go SDK (stdio).

## Technical Context

**Language/Version**: Go (latest stable, 1.23+)
**Primary Dependencies**: plandem/silero-go (VAD+audio), whisper.cpp Go bindings (STT), Claude Code CLI via os/exec (post-processing), modelcontextprotocol/go-sdk (MCP)
**Storage**: Local filesystem (`~/.tatic/`) — markdown files with YAML frontmatter, hierarchical category directories
**Testing**: `go test ./...` (TDD strict per constitution)
**Target Platform**: macOS (primary), Linux (secondary)
**Project Type**: CLI daemon + MCP server (core library + multiple entry points)
**Performance Goals**: Mic activation <3s, STT save <5s after speech ends, AI agent response <30s with 100+ entries
**Constraints**: Negligible CPU/memory during idle (always-on daemon), STT offline-capable (Whisper local), post-processing requires internet (Claude Code CLI → Claude API)
**Scale/Scope**: Single user, 100+ knowledge entries target, single language (Korean)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Check

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. TDD** | PASS | All components (VAD, STT, storage, search, MCP) will follow Red→Green→Refactor. Tests committed with implementation. |
| **II. Simplicity/YAGNI** | PASS | 4 external Go dependencies + Claude Code CLI (runtime). Each justified by a core requirement. No plugin system, no feature flags. Phase 2 (desktop app) deferred. |
| **Tech: Go** | PASS | Go latest stable |
| **Tech: VAD→STT pipeline** | PASS | Silero VAD activates Whisper only on speech detection. No continuous STT streaming. |
| **Tech: Local-first storage** | PASS | All knowledge stored as local markdown files in `~/.tatic/`. No external DB. |
| **Tech: Preprocessing** | PASS | Claude Code CLI generates title/summary/category before storage. Files are query-ready. |
| **Tech: AI Agent Interface** | PASS | MCP server exposes search/list/get tools via official Go SDK. |
| **Tech: Resource Efficiency** | PASS | Silero VAD: 0.43% CPU during continuous monitoring. Whisper only runs on speech segments. |

### Post-Design Check

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. TDD** | PASS | Data model supports testable interfaces. Storage/search/MCP handlers all testable in isolation. |
| **II. Simplicity/YAGNI** | PASS | Flat data model (1 entity + config). No ORM, no search engine, no database. File glob + string matching for search. |
| **Tech: Preprocessing** | PASS | Claude Code CLI (`claude -p --json-schema`) via os/exec. Claude 구독 포함이므로 추가 비용 $0. 응답 시간 ~6-13초/건. |

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
tatic/
├── testdata/
│   └── test_voice_recording.m4a  # E2E 테스트용 음성 파일
├── cmd/
│   └── tatic/           # Single binary entry point (start/stop/status/search/list/mcp)
│       └── main.go
├── pkg/
│   ├── audio/           # Audio capture + VAD integration
│   │   ├── capture.go   # Microphone capture via miniaudio
│   │   ├── decode.go    # Audio file decoder (m4a/wav → 16kHz float32 PCM)
│   │   ├── vad.go       # Silero VAD wrapper
│   │   └── segment.go   # Speech segment buffer management
│   ├── stt/             # Speech-to-text
│   │   ├── whisper.go   # whisper.cpp Go bindings wrapper
│   │   └── model.go     # Model loading and configuration
│   ├── process/         # Post-processing (title/summary/category)
│   │   └── classify.go  # Claude Code CLI invocation via os/exec
│   ├── storage/         # Knowledge file management
│   │   ├── writer.go    # Markdown file writer (frontmatter + content)
│   │   ├── reader.go    # Markdown file reader/parser
│   │   ├── search.go    # File glob + keyword matching search
│   │   └── listing.go   # Directory traversal and listing
│   ├── config/          # Configuration management
│   │   └── config.go    # YAML config loading from ~/.tatic/config.yaml
│   ├── daemon/          # Daemon lifecycle management
│   │   └── pid.go       # PID file management + stale detection
│   └── pipeline/        # Pipeline orchestration
│       └── pipeline.go  # VAD→STT→Process→Store pipeline wiring
├── go.mod
└── go.sum
```

**Structure Decision**: Go idiomatic `cmd/` + `pkg/` layout, matching the spec's "코어 라이브러리 + 다중 진입점" architecture. Each `pkg/` package is independently testable. Single `tatic` binary with subcommands (`start`, `stop`, `status`, `search`, `list`, `mcp`) — the `mcp` subcommand starts the MCP server (stdio transport).

### Daemonization Strategy

`tatic start`는 현재 프로세스에서 파이프라인을 실행한다 (foreground 모드). 백그라운드 실행은 사용자가 `tatic start &` 또는 `nohup tatic start &`로 처리한다. Go는 Unix fork를 네이티브로 지원하지 않으므로, 자체 daemonization은 구현하지 않는다 (YAGNI).

- `tatic start`: 포그라운드에서 파이프라인 실행, PID 파일 기록, Ctrl+C (SIGINT/SIGTERM)로 graceful shutdown
- `tatic stop`: PID 파일에서 프로세스 ID를 읽어 SIGTERM 전송
- `tatic status`: PID 파일 존재 + 프로세스 생존 여부 확인

## Test Strategy

### Test Levels

| Level | Scope | External Deps | 실행 환경 |
|-------|-------|---------------|----------|
| Unit | 패키지 내부 로직 (파싱, 검색, 설정 로드) | 없음 (mock/stub) | `go test ./...` CI/로컬 |
| Integration | 패키지 간 연동 (STT→파일 생성, storage read/write) | Whisper 모델, Claude CLI | 로컬 (시스템 의존성 필요) |
| E2E Pipeline | 오디오 파일 → STT → 후처리 → MD 파일 생성 전체 파이프라인 | Whisper 모델, Claude CLI | 로컬 (시스템 의존성 필요) |

### Test Fixture

**파일**: `testdata/test_voice_recording.m4a` (프로젝트 루트의 `test_voice_recording.m4a`를 `testdata/`로 이동)

이 오디오 파일은 실제 음성 녹음이며, 전체 파이프라인의 통합 테스트에 사용된다. 테스트 실행 시 마이크 접근 없이 파일 기반으로 STT를 수행한다.

### Integration Test: STT (pkg/stt)

```go
// pkg/stt/whisper_integration_test.go
//go:build integration

func TestWhisperTranscribe_RealAudio(t *testing.T) {
    modelPath := os.Getenv("TATIC_WHISPER_MODEL")
    if modelPath == "" {
        t.Skip("TATIC_WHISPER_MODEL not set, skipping integration test")
    }

    // 1. m4a → 16kHz mono float32 PCM 변환
    samples, err := audio.DecodeFile("../../testdata/test_voice_recording.m4a")
    require.NoError(t, err)
    require.NotEmpty(t, samples, "decoded audio samples must not be empty")

    // 2. Whisper STT 실행
    w, err := NewWhisper(modelPath)
    require.NoError(t, err)
    defer w.Close()

    text, err := w.Transcribe(context.Background(), samples)
    require.NoError(t, err)

    // 3. 검증: 비어있지 않은 한국어 텍스트 생성
    assert.NotEmpty(t, text)
    t.Logf("STT result: %s", text)
}
```

**검증 포인트**:
- m4a 오디오 파일을 PCM float32로 디코딩 가능
- Whisper가 비어있지 않은 텍스트를 반환
- 로그로 STT 결과 확인 (정확도는 수동 검증)

### Integration Test: Post-Processing (pkg/process)

```go
// pkg/process/classify_integration_test.go
//go:build integration

func TestClassify_RealCLI(t *testing.T) {
    if _, err := exec.LookPath("claude"); err != nil {
        t.Skip("claude CLI not found, skipping integration test")
    }

    sttText := "Go에서 에러 핸들링할 때 sentinel error 패턴을 쓰면 좋은 게 뭐냐면..."

    result, err := Classify(context.Background(), sttText, nil) // nil = no existing categories
    require.NoError(t, err)

    assert.NotEmpty(t, result.Title)
    assert.NotEmpty(t, result.Summary)
    assert.NotEmpty(t, result.Category)
    assert.LessOrEqual(t, len(result.Title), 100)
    // 카테고리 형식: "대분류" 또는 "대분류/소분류"
    assert.LessOrEqual(t, strings.Count(result.Category, "/"), 1)
    t.Logf("Classified: title=%q, category=%q", result.Title, result.Category)
}
```

### E2E Test: 오디오 → 지식 MD 파일 생성

```go
// pkg/pipeline/pipeline_integration_test.go
//go:build integration

func TestPipeline_AudioFileToKnowledgeEntry(t *testing.T) {
    modelPath := os.Getenv("TATIC_WHISPER_MODEL")
    if modelPath == "" {
        t.Skip("TATIC_WHISPER_MODEL not set")
    }
    if _, err := exec.LookPath("claude"); err != nil {
        t.Skip("claude CLI not found")
    }

    // 임시 지식 DB 디렉토리 사용 (테스트 격리)
    tmpDir := t.TempDir()

    // 1. 오디오 파일 → PCM 디코딩
    samples, err := audio.DecodeFile("../../testdata/test_voice_recording.m4a")
    require.NoError(t, err)

    // 2. STT 실행
    w, err := stt.NewWhisper(modelPath)
    require.NoError(t, err)
    defer w.Close()

    text, err := w.Transcribe(context.Background(), samples)
    require.NoError(t, err)
    require.NotEmpty(t, text, "STT must produce non-empty text")
    t.Logf("STT result: %s", text)

    // 3. Claude Code CLI로 제목/요약/카테고리 생성
    classified, err := process.Classify(context.Background(), text, nil)
    require.NoError(t, err)
    t.Logf("Classified: title=%q, summary=%q, category=%q",
        classified.Title, classified.Summary, classified.Category)

    // 4. 마크다운 지식 파일 저장
    entry := &storage.KnowledgeEntry{
        Title:     classified.Title,
        Category:  classified.Category,
        CreatedAt: time.Now(),
        Summary:   classified.Summary,
        Content:   text,
    }
    filePath, err := storage.Write(tmpDir, entry)
    require.NoError(t, err)
    t.Logf("Written to: %s", filePath)

    // 5. 파일 존재 확인
    _, err = os.Stat(filePath)
    require.NoError(t, err, "knowledge file must exist")

    // 6. 파일 내용 파싱 및 검증
    loaded, err := storage.Read(filePath)
    require.NoError(t, err)

    // 6a. Frontmatter 검증
    assert.Equal(t, entry.Title, loaded.Title)
    assert.Equal(t, entry.Category, loaded.Category)
    assert.False(t, loaded.CreatedAt.IsZero())

    // 6b. Body 검증
    assert.NotEmpty(t, loaded.Summary, "summary section must exist")
    assert.NotEmpty(t, loaded.Content, "content section must exist")

    // 6c. 파일 경로 구조 검증: {baseDir}/{category}/{timestamp}.md
    relPath, _ := filepath.Rel(tmpDir, filePath)
    parts := strings.Split(relPath, string(filepath.Separator))
    assert.GreaterOrEqual(t, len(parts), 2, "path must include category directory")
    assert.True(t, strings.HasSuffix(filePath, ".md"))

    // 6d. 파일 원본 텍스트 확인 (마크다운 형식)
    raw, err := os.ReadFile(filePath)
    require.NoError(t, err)
    content := string(raw)
    assert.Contains(t, content, "---")        // YAML frontmatter delimiter
    assert.Contains(t, content, "title:")     // frontmatter fields
    assert.Contains(t, content, "category:")
    assert.Contains(t, content, "created_at:")
}
```

### 오디오 디코딩 유틸리티

m4a 파일을 Whisper가 필요로 하는 16kHz mono float32 PCM으로 변환하는 헬퍼가 필요하다:

```go
// pkg/audio/decode.go
// DecodeFile reads an audio file (m4a, wav, mp3 등) and returns
// 16kHz mono float32 PCM samples using miniaudio decoder.
func DecodeFile(path string) ([]float32, error)
```

miniaudio는 m4a(AAC), wav, mp3, flac 등을 지원하므로 `plandem/silero-go`의 miniaudio 바인딩 또는 별도의 `gen2brain/malgo` 디코더를 활용한다.

### 테스트 실행 방법

```bash
# Unit tests (의존성 없음, CI에서 실행)
go test ./...

# Integration tests (Whisper 모델 + Claude CLI 필요, 로컬 실행)
TATIC_WHISPER_MODEL=~/.tatic/models/ggml-base.bin \
  go test -tags integration -v -timeout 120s ./...

# 특정 E2E 테스트만 실행
TATIC_WHISPER_MODEL=~/.tatic/models/ggml-base.bin \
  go test -tags integration -v -run TestPipeline_AudioFileToKnowledgeEntry ./pkg/pipeline/
```

### Storage Unit Tests (외부 의존성 없음)

```go
// pkg/storage/writer_test.go (unit test — no build tag)

func TestWriteAndRead_RoundTrip(t *testing.T) {
    tmpDir := t.TempDir()
    entry := &KnowledgeEntry{
        Title:     "테스트 제목",
        Category:  "개발/테스트",
        CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, time.FixedZone("KST", 9*3600)),
        Summary:   "테스트 요약입니다.",
        Content:   "테스트 본문 내용입니다.",
    }

    filePath, err := Write(tmpDir, entry)
    require.NoError(t, err)

    // 디렉토리 구조 검증: tmpDir/개발/테스트/{timestamp}.md
    assert.Contains(t, filePath, filepath.Join("개발", "테스트"))

    loaded, err := Read(filePath)
    require.NoError(t, err)

    assert.Equal(t, entry.Title, loaded.Title)
    assert.Equal(t, entry.Category, loaded.Category)
    assert.Equal(t, entry.Summary, loaded.Summary)
    assert.Equal(t, entry.Content, loaded.Content)
}

func TestWrite_InvalidCategory(t *testing.T) {
    tmpDir := t.TempDir()
    entry := &KnowledgeEntry{
        Title:    "제목",
        Category: "a/b/c", // 3단계 — 최대 2단계 초과
    }
    _, err := Write(tmpDir, entry)
    assert.Error(t, err)
}
```

## Complexity Tracking

> No constitution violations. All design choices are the simplest option that satisfies requirements.
