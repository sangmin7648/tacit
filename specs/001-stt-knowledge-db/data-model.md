# Data Model: STT 기반 지식 DB

**Date**: 2026-03-28 | **Source**: [spec.md](./spec.md), [research.md](./research.md)

## Entities

### 1. KnowledgeEntry (지식 항목)

한 번의 음성 발화에서 생성된 지식 단위. 마크다운 파일로 저장된다.

**File Path Pattern**: `~/.sttdb/{category}/{subcategory?}/{timestamp}.md`
- Example: `~/.sttdb/개발/에러처리/20260328-143052.md`
- Example: `~/.sttdb/잡담/20260328-150000.md`

**YAML Frontmatter Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | Yes | 지식 제목 (Claude Code CLI 자동 생성, ≤100자) |
| `category` | string | Yes | 카테고리 경로 (예: `개발/에러처리`, `잡담`) |
| `created_at` | string (RFC 3339) | Yes | 생성 시각 (예: `2026-03-28T14:30:52+09:00`) |

**Body Sections**:

| Section | Description |
|---------|-------------|
| Summary | 1-2문장 요약 (Claude Code CLI 자동 생성) |
| Content | STT 원문 텍스트 |

**File Format Example**:

```markdown
---
title: "Go 에러 핸들링에서 sentinel error 패턴 사용"
category: "개발/에러처리"
created_at: "2026-03-28T14:30:52+09:00"
---

Go에서 sentinel error를 정의하고 errors.Is로 비교하는 패턴에 대한 논의. 커스텀 에러 타입보다 간단하고 테스트하기 쉽다.

---

어 그래서 Go에서 에러 핸들링할 때 sentinel error 패턴을 쓰면 좋은 게 뭐냐면 errors.Is로 비교할 수 있잖아. 커스텀 에러 타입을 만드는 것보다 훨씬 간단하고 테스트도 쉬워. 특히 패키지 수준에서 var ErrNotFound = errors.New("not found") 이렇게 정의해놓으면 호출하는 쪽에서 깔끔하게 처리할 수 있거든.
```

**Go Struct**:

```go
type KnowledgeEntry struct {
    Title     string    `yaml:"title"`
    Category  string    `yaml:"category"`
    CreatedAt time.Time `yaml:"created_at"`
    Summary   string    // Body first section (before ---)
    Content   string    // Body second section (after ---)
    FilePath  string    // Absolute file path (not stored in file, derived)
}
```

**Validation Rules**:
- `title`: non-empty, max 100 characters
- `category`: max 2 levels deep (e.g., `대분류` or `대분류/소분류`), no leading/trailing slashes
- `created_at`: valid RFC 3339 timestamp
- `summary`: non-empty
- `content`: non-empty (minimum utterance length already enforced by VAD)

**State Transitions**: None. Knowledge entries are immutable once created. Users may manually edit files via filesystem.

---

### 2. Config (설정)

사용자 설정. `~/.sttdb/config.yaml`에 저장. 파일이 없으면 기본값으로 동작.

**File Path**: `~/.sttdb/config.yaml`

**Fields**:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `whisper_model` | string | `"base"` | Whisper 모델 크기 (`tiny`, `base`, `small`, `medium`, `large`) |
| `min_speech_duration` | duration | `3s` | 최소 발화 길이 임계값 (이하 무시) |
| `silence_duration` | duration | `1.5s` | 발화 종료 판단 침묵 길이 |
| `speech_threshold` | float | `0.5` | VAD 음성 감지 임계값 (0.0-1.0) |
| `claude_model` | string | `"haiku"` | Claude Code CLI 후처리에 사용할 모델 (`haiku`, `sonnet`, `opus`) |

**Go Struct**:

```go
type Config struct {
    WhisperModel     string        `yaml:"whisper_model"`
    MinSpeechDur     time.Duration `yaml:"min_speech_duration"`
    SilenceDuration  time.Duration `yaml:"silence_duration"`
    SpeechThreshold  float64       `yaml:"speech_threshold"`
    ClaudeModel       string        `yaml:"claude_model"`
}
```

**Config File Example**:

```yaml
whisper_model: small
min_speech_duration: 2s
silence_duration: 2s
speech_threshold: 0.6
claude_model: haiku
```

---

### 3. AudioSegment (음성 구간) — In-Memory Only

VAD에 의해 감지된 연속 음성 구간. 메모리에서만 존재하며 파일로 저장되지 않는다.

```go
type AudioSegment struct {
    Samples   []float32 // 16kHz mono PCM samples
    StartTime time.Time // Segment start timestamp
    Duration  time.Duration
}
```

---

## Knowledge Base Directory Structure

```text
~/.sttdb/
├── config.yaml                         # 사용자 설정 (optional)
├── sttdb.pid                           # 데몬 PID 파일 (runtime)
├── 개발/
│   ├── 에러처리/
│   │   ├── 20260328-143052.md
│   │   └── 20260328-160230.md
│   └── 테스트/
│       └── 20260329-091500.md
├── 디자인/
│   └── UX패턴/
│       └── 20260328-151000.md
├── 회의/
│   └── 20260328-100000.md
└── 잡담/
    └── 20260328-120000.md
```

**Directory Rules**:
- Max 2 levels: `{대분류}/` or `{대분류}/{소분류}/`
- Categories auto-created by Claude Code CLI classification
- No predefined category list — AI determines optimal category from existing structure
- `잡담/` is a parallel category at the same level as work categories
- File naming: `{YYYYMMDD}-{HHmmss}.md` (timestamp of speech end)

## Relationships

```text
Config (1) ←──── controls ────→ Pipeline (1)
Pipeline (1) ←── produces ────→ KnowledgeEntry (many)
AudioSegment (1) ←── transforms to ──→ KnowledgeEntry (1)
```

- One Config controls one Pipeline instance
- One Pipeline produces many KnowledgeEntries over time
- Each AudioSegment transforms into exactly one KnowledgeEntry (or is discarded if too short)
