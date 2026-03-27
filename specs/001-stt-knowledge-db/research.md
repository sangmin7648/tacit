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

**Recommendation**: Start with `base` as default per spec (speed/size balance), but design STT interface to be model-agnostic. Users switch via `~/.sttdb/config.yaml`. Document that `small` or fine-tuned Korean models are recommended for better Korean accuracy.

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

**Decision**: Anthropic Go SDK (`anthropic-sdk-go`) calling Haiku 4.5 directly

**Rationale**:
- ~100x cheaper than Claude Code CLI: ~$0.001 vs ~$0.04-0.15 per entry
- No process spawn overhead (direct HTTP vs forking Node.js process)
- Built-in retry logic (2 retries for 429, 5xx)
- Proper rate limit handling via response headers
- Concurrency control with goroutines

**Alternatives Considered**:
| Alternative | Why Rejected |
|-------------|-------------|
| Claude Code CLI (`claude -p --json-schema`) | Works well (verified with Korean text), but ~$0.04-0.15 per call due to CLI's ~35K token system prompt overhead. Impractical for frequent automated calls. |
| Local LLM for classification | Adds another large model to memory alongside Whisper. Violates simplicity principle. |
| No AI post-processing (manual) | Defeats the purpose of automatic knowledge organization |

**Verified CLI Capabilities** (useful for future features/fallback):
- `--print` + `--output-format json` + `--json-schema` produces structured output in `structured_output` field
- `--tools ""` disables all tools for pure text processing
- `--bare` skips hooks/plugins for fastest startup (requires `ANTHROPIC_API_KEY`)
- `--model haiku` selects cheaper model
- Korean text classification works correctly (잡담 vs 회의 vs 업무 etc.)

**Combined Schema** (single API call for both tasks):
```json
{
  "title": "string (concise, Korean, <50 chars)",
  "summary": "string (1-2 sentences, Korean)",
  "category": "string (auto-determined from existing categories)"
}
```

**Cost Estimate**: ~$0.001 per knowledge entry with Haiku 4.5 (~200-500 input tokens + ~50-100 output tokens)

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

**Decision**: PID file (`~/.sttdb/sttdb.pid`) with stale PID detection

**Rationale**:
- Simplest daemon management approach for a single-user local service
- Standard Unix pattern — check PID file, verify process exists via `os.FindProcess` + signal 0
- Automatic cleanup of stale PIDs (process crashed/killed)
- No dependency on systemd, launchd, or other init systems

**Implementation**: `sttdb start` forks daemon, writes PID. `sttdb stop` reads PID, sends SIGTERM. `sttdb status` checks PID liveness.

---

## Dependency Summary

| Dependency | Purpose | Justification |
|-----------|---------|---------------|
| `plandem/silero-go` | VAD + audio capture | Core pipeline requirement, bundles VAD + miniaudio |
| ONNX Runtime (~17 MB) | Silero VAD inference | Required by Silero VAD model |
| `ggml-org/whisper.cpp` (Go bindings) | Local STT | Core pipeline requirement, offline Whisper |
| `anthropic-sdk-go` | Text post-processing | Title/summary/category generation, ~100x cheaper than CLI |
| `modelcontextprotocol/go-sdk` | MCP server | Official SDK for AI agent interface |
| Go standard library | Config, filesystem, HTTP, signals | No additional deps needed for these |

Total external dependencies: 5 (each justified by a core requirement)
