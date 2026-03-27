<!--
## Sync Impact Report
- **Version change**: N/A → 1.0.0 (initial ratification)
- **Modified principles**: None (first version)
- **Added sections**:
  - Core Principles: I. Test-First (TDD), II. Simplicity / YAGNI
  - Technology Constraints (Go, local-first, audio pipeline)
  - Development Workflow (TDD strict, commit discipline)
  - Governance
- **Removed sections**: None
- **Templates requiring updates**:
  - `.specify/templates/plan-template.md` — ✅ No changes needed (Constitution Check section is generic)
  - `.specify/templates/spec-template.md` — ✅ No changes needed (user stories align with TDD workflow)
  - `.specify/templates/tasks-template.md` — ✅ No changes needed (already enforces tests-before-implementation)
  - `.specify/templates/commands/*.md` — ✅ No command templates found
- **Follow-up TODOs**: None
-->

# STT Database (sttdb) Constitution

## Core Principles

### I. Test-First (TDD) — NON-NEGOTIABLE

All production code MUST be written using strict Test-Driven Development:

- **Red**: Write a failing test that defines the desired behavior.
- **Green**: Write the minimum code to make the test pass.
- **Refactor**: Clean up while keeping tests green.

Rules:
- No production code without a corresponding failing test first.
- Tests MUST be committed before (or in the same commit as) the
  implementation they validate.
- Test coverage gaps MUST be justified in the PR description.
- `go test ./...` MUST pass on every commit.

**Rationale**: sttdb processes ambient audio into a persistent
knowledge store. Bugs in the pipeline (VAD, STT, preprocessing,
storage) silently corrupt knowledge. TDD catches regressions early
and documents intended behavior as executable specifications.

### II. Simplicity / YAGNI

Every addition MUST earn its place:

- Start with the simplest implementation that satisfies the current
  requirement. Do not build for hypothetical future needs.
- Prefer standard library over third-party dependencies. Each new
  dependency MUST be justified.
- Three similar lines of code are better than a premature abstraction.
- No feature flags, plugin systems, or extensibility hooks unless a
  concrete, current use case demands them.
- If a design decision feels complex, document *why* the complexity
  is unavoidable in a code comment or PR description.

**Rationale**: sttdb operates as a local, always-on service.
Unnecessary complexity increases resource consumption, attack
surface, and maintenance burden on a system that MUST be reliable
and lightweight.

## Technology Constraints

- **Language**: Go (latest stable release).
- **Audio Pipeline**: VAD (Voice Activity Detection) activates STT
  only when speech is detected. The system MUST NOT continuously
  stream to an STT engine when no speech is present.
- **Storage**: Local-first. All transcribed and preprocessed data
  MUST be stored on the local filesystem or an embedded database.
  No mandatory external service dependencies at runtime.
- **Preprocessing**: Raw STT output MUST be preprocessed (cleaned,
  structured, indexed) before storage so it is query-ready for AI
  agents.
- **AI Agent Interface**: The knowledge store MUST expose a clear
  retrieval interface (CLI, API, or library) that AI agents can
  call to obtain contextual knowledge.
- **Resource Efficiency**: As an always-on background service, CPU
  and memory usage during idle (no speech) MUST be negligible.

## Development Workflow

- **Branching**: One branch per feature or fix, branched from `main`.
- **TDD Cycle**: Every feature follows Red → Green → Refactor.
  Implementation PRs that lack tests MUST be rejected.
- **Commit Discipline**: Each commit MUST represent a single logical
  change. Prefer small, focused commits over large batches.
- **Code Review**: All code merges to `main` via pull request.
  Reviewer MUST verify TDD compliance and simplicity adherence.
- **CI**: `go test ./...` and `go vet ./...` MUST pass before merge.
  Linting (`golangci-lint`) is strongly recommended.

## Governance

This constitution is the highest-authority document for sttdb
development practices. All other guidelines, templates, and ad-hoc
decisions are subordinate.

- **Amendments**: Any change to this constitution MUST be documented
  with a version bump, rationale, and migration plan for affected
  code or processes.
- **Versioning**: MAJOR for principle removals/redefinitions, MINOR
  for new principles or material expansions, PATCH for wording
  and clarification fixes.
- **Compliance**: Every PR and code review MUST verify adherence to
  the principles above. Non-compliance MUST be flagged and resolved
  before merge.

**Version**: 1.0.0 | **Ratified**: 2026-03-28 | **Last Amended**: 2026-03-28
