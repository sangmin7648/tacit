# tacit

tacit turns spoken words into a searchable local knowledge base: it listens to the microphone, transcribes on-device, classifies each transcript with an LLM, and stores it as a Markdown file that AI agents can search.

**Read `docs/index.md` first.** It lists every document and what it covers. The code is the source of truth; `docs/` explains the *why* and the *shape* at a high level.

## Build rules

- Never verify with `go build ./...`. `pkg/stt` reaches whisper.cpp through CGo and fails until `make build` has produced the libraries. Use `make build`, `make test`, and `make e2e-test`.
- After changing pipeline behaviour, run `make e2e-test`.
- Everything except the AI agent (the Claude Code CLI) is linked into the binary. The build output must be portable with no runtime library installs.

## Code comments

Comments are a liability: they go stale silently and cost reading time. Make the code readable through names and structure first; reach for a comment only when the code cannot carry the point.

- **Don't** write comments that restate what the code does, or narrate it step by step. If a block needs that, rename or extract instead.
- **Do**, occasionally, explain a broad or tricky concept in plain words, where a reader would otherwise have to reconstruct it from many places.
- **Do**, occasionally, say *why* the code is written this way: the failure it prevents, the constraint it works around, the option rejected.
- When touching code, delete comments that no longer earn their place.

## Docs: what they are

`docs/` is a small wiki kept by the agent. It holds only three kinds of content:

1. **Concepts and packages**: what a thing is for, how the parts fit, and the invariants that must keep holding.
2. **Decisions**: why we chose one path over another, written as records that are never rewritten.
3. **Domain terms**: the vocabulary the project uses (`docs/glossary.md`).

It never holds code-level detail: no function signatures, struct fields, line numbers, or walkthroughs. If a sentence would go stale on a refactor that changes no behaviour, it does not belong in `docs/`. Link to a file path instead of describing its contents.

Layout:

```
docs/index.md       every page, one line each, with the commit it was last checked against
docs/log.md         append-only record of doc changes
docs/overview.md    the whole system in one page
docs/glossary.md    domain terms
docs/concepts/      one page per idea that spans packages
docs/packages/      one page per package or binary under pkg/, cmd/, skills/
docs/decisions/     NNNN-title.md, one decision each
```

## Page format

Every page under `docs/` starts with front matter:

```yaml
---
source: [pkg/pipeline, pkg/process]   # code the page describes; paths, not symbols
verified: 9825e8e                      # commit at which the page was last checked
---
```

Decision pages use `status: accepted | superseded` and `date:` instead, plus `superseded-by:` when relevant.

Keep a page to one topic. Link related pages with relative links. Prefer a short page over a long one.

## Operations

**Ingest** (after a code change, or when asked to document something)
1. Find pages whose `source` paths the change touched. Read those pages and the diff.
2. Update only what the change made untrue. Bump `verified` on every page you checked, even if unchanged.
3. If the change reverses or replaces a decision, add a new decision page and mark the old one `superseded`. Never edit the reasoning of an accepted decision.
4. If the change introduces a concept, package, or term with no page, add it.
5. Update `docs/index.md` and append a line to `docs/log.md`.

**Query** (when asked how something works)
Read `docs/index.md`, open the relevant pages, then confirm against the code before answering. If the docs and the code disagree, say so and fix the docs.

**Lint** (periodic, or when asked)
1. For each page, compare `verified` with `git log` on its `source` paths; list pages whose sources changed since.
2. Check claims against the code, and flag contradictions between pages.
3. Check that every page is in the index, every link resolves, and no page contains code-level detail.
4. Report findings first; fix only what is clear-cut. Log the pass in `docs/log.md`.

## Writing style

- English, plain, present tense. State the invariant, then the reason.
- Name the failure a rule prevents. "X exists because Y went wrong" ages better than "X does Y".
- Do not duplicate `README.md`: user-facing install and usage live there, and the per-field setting reference lives there too; `docs/concepts/configuration.md` explains only how settings are layered and grouped.
