# Research: STT 기반 지식 DB 자동 구축

**Date**: 2026-03-28
**Status**: Complete

## Decision 1: Voice Activity Detection (VAD)

**Decision**: `plandem/silero-go` (Silero VAD v5/v6 via ONNX Runtime + built-in miniaudio capture)

**Rationale**:
- Silero VAD v6 achieves ROC-AUC 0.97 — at 5% FPR, 87.7% TPR (4x fewer errors than WebRTC VAD)
- Built-in miniaudio capture eliminates the need for a separate audio library
- Callback-based API (`OnSpeechSegmentCallback`) maps directly to the VAD→STT pipeline
- CPU: <1ms per 30ms chunk, 0.43% CPU for continuous real-time — negligible for always-on daemon
- Config knobs (`SpeechThreshold`, `MinSilence`, `SpeechPad`) directly address minimum utterance duration requirements

**Alternatives Considered**:
| Alternative | Why Rejected |
|-------------|-------------|
| `streamer45/silero-vad-go` | More mature (77 stars, Mattermost production) but no built-in audio capture — requires separate malgo dependency. Kept as fallback. |
| `TEN-framework/ten-vad` | Smallest footprint (306 KB vs 17 MB ONNX), but Go support is recent, accuracy claims not independently verified |
| WebRTC VAD (`go-webrtcvad`) | ~50% TPR at 5% FPR — unacceptable miss rate for always-on capture |
| Energy-based VAD | Too simplistic, cannot distinguish speech from noise |

**Mitigation for newness risk**: `plandem/silero-go` is ~200 lines of wrapper. If problematic, migrate to `streamer45/silero-vad-go` + `gen2brain/malgo` (same underlying Silero ONNX model, low migration cost).

**Dependencies**: ONNX Runtime shared library (~17 MB) + Silero ONNX model (~2 MB)

---

## Decision 2: Speech-to-Text (STT)

**Decision**: `ggml-org/whisper.cpp/bindings/go/pkg/whisper` (official whisper.cpp Go CGo bindings)

**Rationale**:
- Direct CGo integration gives lowest latency for real-time VAD→STT pipeline
- Processes `[]float32` samples directly — no intermediate file I/O
- Single binary output — no external process dependencies
- Under `ggml-org` organization with active whisper.cpp maintenance

**Alternatives Considered**:
| Alternative | Why Rejected |
|-------------|-------------|
| `mutablelogic/go-whisper` | Feature-rich HTTP server wrapper, but adds HTTP overhead for in-process use. Kept as fallback (HTTP server mode). |
| `kardianos/whisper.cpp` | Vendored C source — stale since 2023, misses 2+ years of improvements |
| whisper-server subprocess | Process startup overhead, harder single-binary distribution |
| Cloud STT API | Spec requires offline/local operation |

**Korean Language Accuracy Warning**:
| Model | Size | RAM | Korean CER (estimated) | Speed |
|-------|------|-----|----------------------|-------|
| `base` (default) | 142 MB | ~388 MB | ~20-30% | ~7x real-time |
| `small` | 466 MB | ~852 MB | ~10-18% | ~4x real-time |
| `small` (fine-tuned Korean) | ~484 MB | ~852 MB | ~6.45% | ~4x real-time |

**Recommendation**: Start with `base` as default per spec (speed/size balance), but design STT interface to be model-agnostic. Users switch via `~/.tatic/config.yaml`. Document that `small` or fine-tuned Korean models are recommended for better Korean accuracy.

**Audio Format**: 16 kHz, mono, float32 (normalized [-1.0, 1.0]). Convert int16 samples by dividing by 32768.0.

---

## Decision 3: Audio Capture

**Decision**: Built-in miniaudio via `plandem/silero-go` (primary), `gen2brain/malgo` as standalone fallback

**Rationale**:
- `plandem/silero-go` includes miniaudio capture that feeds directly into VAD detector
- miniaudio has no system library dependency on macOS (vendored via CGo)
- Supports 16 kHz mono capture for Whisper compatibility
- Cross-platform: macOS, Linux, Windows

**Alternatives Considered**:
| Alternative | Why Rejected |
|-------------|-------------|
| `gordonklaus/portaudio` | Requires PortAudio system library installation — violates "no extra setup" goal (SC-006) |
| Raw CoreAudio/ALSA | Too low-level, not cross-platform |

**macOS Microphone Permission**: macOS requires user consent for microphone access. The Go binary will trigger the system permission dialog on first run. If denied, the program must detect this and show a clear error (FR-009).

---

## Decision 4: Text Post-Processing (Title/Summary/Category Generation)

**Decision**: Claude Code CLI (`claude -p --json-schema`) via `os/exec`

**Rationale**:
- 사용자가 Claude 구독 중이므로 CLI 사용은 추가 비용 $0 (구독에 포함)
- `--json-schema` 플래그로 structured output (title/summary/category) 자동 추출 검증 완료
- `--model haiku` 으로 가벼운 모델 선택 가능
- `--tools ""` 로 불필요한 tool 사용 방지
- Korean 텍스트 분류 정확도 검증 완료 (잡담 vs 회의 vs 업무 등)

**Alternatives Considered**:
| Alternative | Why Rejected |
|-------------|-------------|
| Anthropic Go SDK (`anthropic-sdk-go`) | API 호출 당 ~$0.001이지만 별도 API 키 및 과금 필요. Claude 구독으로 CLI가 무료이므로 불필요한 추가 비용. |
| Local LLM for classification | Whisper와 함께 또 다른 대형 모델을 메모리에 올림. 단순성 원칙 위반. |
| No AI post-processing (manual) | 자동 지식 정리라는 핵심 목적에 반함 |

**CLI Invocation Pattern**:
```bash
echo "<STT text>" | claude -p \
  --output-format json \
  --json-schema '{"type":"object","properties":{"title":{"type":"string"},"summary":{"type":"string"},"category":{"type":"string"}},"required":["title","summary","category"]}' \
  --model haiku \
  --tools "" \
  --no-session-persistence
```

**Go Integration** (`os/exec`):
```go
cmd := exec.CommandContext(ctx, "claude", "-p",
    "--output-format", "json",
    "--json-schema", schemaJSON,
    "--model", "haiku",
    "--tools", "",
    "--no-session-persistence",
)
cmd.Stdin = strings.NewReader(sttText)
output, err := cmd.Output()
// Parse JSON → extract .structured_output.{title, summary, category}
```

**Verified Capabilities**:
- `--print` + `--output-format json` + `--json-schema` → `structured_output` 필드에 파싱된 결과
- `--tools ""` → 순수 텍스트 처리 (Bash/Read/Edit 도구 비활성화)
- `--model haiku` → 가벼운 모델로 빠른 응답
- Korean 분류 테스트: "김치찌개 점심" → `잡담`, "프로젝트 일정 회의" → `회의` 정확 분류 확인

**Combined Schema** (single CLI call for all tasks):
```json
{
  "title": "string (concise, Korean, <50 chars)",
  "summary": "string (1-2 sentences, Korean)",
  "category": "string (auto-determined from existing categories)"
}
```

**Cost Estimate**: $0 per entry (Claude 구독 포함). CLI 응답 시간 ~6-13초/건.

---

## Decision 5: MCP Server

**Decision**: `modelcontextprotocol/go-sdk` (official MCP Go SDK) with stdio transport

**Rationale**:
- Official SDK, v1.0+ stable API guarantee, Google co-maintained
- Go generics-based type safety (`mcp.AddTool[In, Out]`) with automatic JSON schema inference from struct tags
- Stdio transport is correct for local-only service (no network, no auth needed)
- Clean separation: thin `cmd/mcp/` entry point calls into `pkg/` core library

**Alternatives Considered**:
| Alternative | Why Rejected |
|-------------|-------------|
| `mark3labs/mcp-go` | 8.4K stars, 4 transports, but pre-1.0 API — breaking changes possible |
| HTTP/SSE transport | Unnecessary complexity for local-only tool |
| Custom JSON-RPC | Reinventing the wheel when official SDK exists |

**3 Tools to Expose**:
1. `search_knowledge(query, category?, include_chitchat?)` — keyword matching + category filtering
2. `list_knowledge(category?, date_range?)` — knowledge listing
3. `get_knowledge(id)` — detail retrieval

**Client Registration**: `.mcp.json` at project root for team sharing + `claude mcp add --scope user` for personal use.

---

## Decision 6: Daemon Management

**Decision**: PID file (`~/.tatic/tatic.pid`) with stale PID detection

**Rationale**:
- Simplest daemon management approach for a single-user local service
- Standard Unix pattern — check PID file, verify process exists via `os.FindProcess` + signal 0
- Automatic cleanup of stale PIDs (process crashed/killed)
- No dependency on systemd, launchd, or other init systems

**Implementation**: `tatic start` forks daemon, writes PID. `tatic stop` reads PID, sends SIGTERM. `tatic status` checks PID liveness.

---

## Dependency Summary

| Dependency | Purpose | Justification |
|-----------|---------|---------------|
| `plandem/silero-go` | VAD + audio capture | Core pipeline requirement, bundles VAD + miniaudio |
| ONNX Runtime (~17 MB) | Silero VAD inference | Required by Silero VAD model |
| `ggml-org/whisper.cpp` (Go bindings) | Local STT | Core pipeline requirement, offline Whisper |
| Claude Code CLI (`claude`) | Text post-processing | Title/summary/category generation via `os/exec`. Claude 구독 포함 ($0 추가 비용). |
| `modelcontextprotocol/go-sdk` | MCP server | Official SDK for AI agent interface |
| Go standard library | Config, filesystem, HTTP, signals | No additional deps needed for these |

Total external Go dependencies: 4 (+ Claude Code CLI as runtime dependency)
