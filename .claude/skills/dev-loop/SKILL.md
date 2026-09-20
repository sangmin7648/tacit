---
name: dev-loop
description: |
  Runs an autonomous plan -> build -> verify loop for developing the tacit codebase itself (repo contributors only — never installed for tacit CLI end users). Drives a task end-to-end against this repo's real gates (`make build` + `make test` = CI parity, `make e2e-test` for pipeline changes), iterating diagnose -> fix -> re-gate until green or a bounded iteration cap, then stops for human review before any commit/push.
  TRIGGER when: user invokes /dev-loop, or says things like "loop on this", "keep iterating until it builds/passes", "drive this to green", or describes a bug/feature in this repo and wants it implemented and verified end-to-end without a check-in after every step.
  DO NOT TRIGGER when: user wants a single change reviewed step-by-step, is only asking a question, or the task isn't a code change to this repo.
---

# tacit Dev Loop

Adapts the loop from the [AI-native SDLC playbook](https://claude.com/blog/the-ai-native-sdlc-playbook) — intent -> plan -> build -> verify -> human gate — to local development on the tacit codebase. Two rules carry the whole thing:

1. **Every iteration ends at a gate that could actually fail.** A green you can get without doing the work is not evidence.
2. **The loop stops at the human gate.** You produce a verified diff; the user decides whether it ships.

**Not for end users:** the root-level [`skills/`](../../../skills) directory is a separate thing — it is embedded into the `tacit` binary and installed into *end users'* `~/.claude/skills/` via `tacit install-skills`/`tacit setup`. Never touch that directory or its skills (`tacit.knowledge`, `tacit.memorize`) as part of this loop; they ship with the product and are unrelated to developing it.

## 0. Preflight — is this checkout buildable at all?

Do this once per worktree, before writing code. A fresh worktree is **not** build-ready, and finding out three iterations in wastes the budget.

| Check | Command | If it fails |
|---|---|---|
| whisper.cpp submodule present | `git submodule status` (leading `-` = not initialized) | `git submodule update --init --recursive` |
| ripgrep binaries for `pkg/search`'s `go:embed` | `ls pkg/search/rg-darwin-*` | `make rg-download` (network) |
| cmake on PATH | `command -v cmake` | `brew install cmake` |
| Claude CLI (only for `make e2e-test`) | `command -v claude` | environment problem — report it, don't code around it |

First `make build` in a clean worktree compiles whisper.cpp from scratch (minutes). Later builds are incremental. Never `make clean` as a debugging reflex — it throws that away.

## 1. Capture intent

State in 1–3 sentences what "done" looks like, phrased as something a gate can decide: "`tacit process` on the fixture stores an entry instead of skipping", "`TestTranscriptDeduper` covers the N-th duplicate", not "dedup works better". If a failing build/test output triggered this, that output *is* the intent.

## 2. Plan (read-only)

Find the root cause before editing. Read the relevant package (see [CLAUDE.md](../../../CLAUDE.md)'s Project Structure), and read the failing output rather than pattern-matching on the symptom. Keep the plan in your response — no planning doc — and keep the eventual change lean: smallest change that satisfies the intent, no speculative abstractions, no unrelated cleanup.

## 3. Build

Implement that change. If the diff starts growing past the intent, that is the signal the diagnosis was wrong — go back to step 2 rather than widening the patch.

## 4. Verify — the gate ladder

Run the cheapest gate that can disprove the change, then climb. Never `go build ./...`: `pkg/stt` pulls in `whisper.h` via CGo and it always fails outside `make`.

| Gate | Proves | Cost | When |
|---|---|---|---|
| `go test ./pkg/config/ ./pkg/daemon/ ./pkg/process/ ./pkg/storage/` | pure-Go logic (no CGo, no whisper libs) | ~2s | inner loop for changes in those packages |
| `go build ./skills/` | a CGo-free package still compiles | ~1s | changes isolated to it |
| `make build` | the real compile: CGo, whisper.cpp, ten-vad bundling | incremental | **any Go change** |
| `make test` | `go test ./...`, including CGo packages | fast once built | **any Go change** |
| `make e2e-test` | build + real audio through the pipeline + `-tags integration` classifier test (spends Claude CLI tokens) + darwin speaker test | minutes | any change that can reach the capture -> VAD -> STT -> process -> store runtime |

**CI parity is `make build` + `make test`** (see [.github/workflows/ci.yml](../../../.github/workflows/ci.yml), macos-15). The inner-loop `go test` on pure-Go packages is a supplement, never a substitute — a change verified only that way can still break CI.

`make e2e-test` is the gate CLAUDE.md asks for after code changes. Run it **once, at the end**, not every iteration: it costs tokens and writes a real entry into `~/.tacit/`. Skip it only for docs-, skills-, or test-only changes, and say so in the report.

## 5. Read the gate honestly

A passing exit code is not a passing gate here:

- `./tacit process <file>` **exits 0 when the classifier skips** ([cmd/tacit/main.go:439](../../../cmd/tacit/main.go)). The `make e2e-test` step is green either way. Decide *before* running which line you need in stdout: `Knowledge entry created: <path>` (pipeline stored something) or `Content classified as meaningless, skipping.` (ran, stored nothing). For a storage/classify/dedup change the skip line is a **red**.
- The darwin speaker test `t.Skipf`s when Screen Recording permission is missing. A skip is not a pass — say which it was.
- Go caches test results. Re-run with `-count=1` when the input changed outside Go source.
- `tacit process` deliberately `os.Exit(0)`s to dodge a ggml Metal cleanup crash — output printed, then immediate exit, is normal, not a truncated run.

Quote the decisive line of output in your report. Not "e2e passed" — the line.

## 6. Environment red vs code red

Triage before you edit. An environment red **does not consume an iteration**, and must never be "fixed" in code.

| Signature | Cause | Fix |
|---|---|---|
| `does not appear to contain CMakeLists.txt` | submodule not initialized in this worktree | `git submodule update --init --recursive` |
| `pattern rg-darwin-amd64: no matching files found` / `[setup failed]` | `pkg/search` embed inputs missing | `make rg-download` |
| `whisper.h: No such file or directory` | bare `go build`/`go test` outside `make` | use `make build` / `make test` |
| `Error: cmake is required` | toolchain missing | `brew install cmake` |
| `claude CLI failed` / `executable file not found` | Claude CLI missing or unauthenticated | environment — report, don't change code |
| long stall on first `make e2e-test` | whisper model downloading into `~/.tacit/models` (GB-scale) | wait once; it is cached |
| `Stream: ...` + skip in `TestSpeaker_Stream_E2E` | Screen Recording permission | grant it, or report the gate as skipped |

## 7. Loop

- One hypothesis per iteration, and the evidence for it quoted from the actual failure output before you edit. No guess-and-retry.
- Keep a short ledger in your response: iteration / hypothesis / evidence / change / gate result.
- Cap at **5 iterations**. Stop earlier if the same gate fails the same way twice — that means the diagnosis is wrong, not the fix.
- If the diff has drifted from the intent, revert your own edits (`git diff` to see them, `git checkout -- <file>`) and restart from the evidence. Never bare `git stash` in a worktree.
- On giving up: report the failure, the verbatim output, and your best diagnosis. A well-diagnosed red beats a mystery green.

## 8. Never weaken the gate

This repo has already had to fix a case where a gate quietly threw away work (commit `e0a274f`, "stop discarding successfully transcribed speech" — classification was acting as a gatekeeper). Do not add another:

- Do not edit, delete, skip, or loosen a test to get green; do not add build tags to dodge one. If a test is genuinely wrong, stop and say so — that is a human call.
- Do not widen a denylist / threshold / filter just to make the fixture pass.
- Do not touch the root `skills/` directory.
- Do not delete or rewrite `~/.tacit/` user data. `make e2e-test` writes a real knowledge entry each run; prune only the entries your own runs created (the path is printed).
- No `git commit`, `git push`, PR, tag, or `make install`.

## 9. Human gate — the report

Once green, stop and report:

1. **Intent** and the done-assertion from step 1.
2. **Change**: files touched and why, in one line each.
3. **Gates**: exact commands run, and the decisive output line from each. State CI parity explicitly (`make build` + `make test` are what CI runs) and name any gate you skipped, with the reason.
4. **Left out**: anything noticed but deliberately out of scope.
5. **Loop feedback**: if a trap cost you an iteration and it is documented neither in CLAUDE.md nor here, propose the one-line addition. Propose — the user edits the repo's institutional knowledge, not you.
