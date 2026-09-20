---
name: dev-loop
description: |
  Runs an autonomous plan -> build -> test loop for developing the tacit codebase itself (repo contributors only — never installed for tacit CLI end users). Drives a task end-to-end against this repo's real build gates (`make build` / `make e2e-test`), iterating diagnose -> fix -> rebuild -> retest until green or a bounded iteration cap, then stops for human review before any commit/push.
  TRIGGER when: user invokes /dev-loop, or says things like "loop on this", "keep iterating until it builds/passes", "drive this to green", or describes a bug/feature in this repo and wants it implemented and verified end-to-end without a check-in after every step.
  DO NOT TRIGGER when: user wants a single change reviewed step-by-step, is only asking a question, or the task isn't a code change to this repo.
---

# tacit Dev Loop

Adapts the "loop" pattern from the [AI-native SDLC playbook](https://claude.com/blog/the-ai-native-sdlc-playbook) — diagnose -> plan -> build -> test -> gate — to local development on the tacit codebase. This is a contributor tool for iterating on tacit's own source.

**Not for end users:** the root-level [`skills/`](../../skills) directory is a separate thing — it's embedded into the `tacit` binary and installed into *end users'* `~/.claude/skills/` via `tacit install-skills`/`tacit setup`. Never touch that directory or its skills (`tacit.knowledge`, `tacit.memorize`) as part of this loop; they ship with the product and are unrelated to developing it.

## Process

### 1. Capture intent
State in 1-3 sentences what "done" looks like (bug fixed, feature behaves as X, test Y passes). If a failing build/test output is what triggered this, treat that output as the intent.

### 2. Plan (read-only)
Explore the relevant packages (see [CLAUDE.md](../../CLAUDE.md)'s Project Structure) and identify the root cause or approach before editing. No separate planning doc — keep it in your head/response, and keep the eventual change lean: no speculative abstractions, no unrelated cleanup.

### 3. Build
Implement the smallest change that satisfies the intent.

### 4. Test — the real gate
Run the verification that matches what you touched:
- `make build` — the default gate for any Go change. Never verify with bare `go build ./...`; `pkg/stt` pulls in `whisper.h` via CGo and that always fails outside `make build`.
- `make e2e-test` — required whenever you touch the capture -> VAD -> STT -> process -> store pipeline; it builds and then runs the real audio fixture through it.
- `go build ./skills/...` (or another pure-Go package) is a fine fast inner-loop check for changes fully isolated to a CGo-free package, but it's a supplement, not a replacement for the `make` gate above.

### 5. Loop
If the gate fails: read the actual failure output and diagnose the real cause — don't guess-and-retry. Fix, then re-run step 4. Cap at 5 iterations; if still red after 5, stop and report the failure plus your best diagnosis instead of continuing to churn.

### 6. Gate for human review
Once green, stop. Do not commit, push, or open a PR yourself — that decision stays with the user, same as the blog's approval-tier model keeps "Deploy" behind human judgment. Report: what changed, which `make` target verified it, and anything noticed but left out of scope.
