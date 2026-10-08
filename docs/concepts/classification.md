---
source: [pkg/process, pkg/pipeline, pkg/storage]
verified: 9825e8e
---

# Classification

Classification asks an LLM to turn a raw transcript into a note: a title, one category, keywords, and a one-sentence summary. It may instead answer **skip**.

## Providers

Two interchangeable backends sit behind one interface: a local Ollama model, or the Claude Code CLI. The user picks one in setup; the choice is checked for reachability before it is saved.

## What the classifier is given

The transcript, plus the categories that already exist in the knowledge base, so it reuses them instead of inventing near-duplicates.

## Three outcomes

1. **Usable result**: stored as an entry.
2. **Skip**: the model judged the text meaningless (filler, bare acknowledgements, call-connection chatter). The one deliberate discard; the dropped text is logged so it can be audited.
3. **Failure or partial result**: the model errored, returned nothing usable, or filled only some fields. The entry is repaired and stored. A missing title is taken from the opening words, a missing category becomes `unsorted`.

## Batching and retry

When several items are queued the worker classifies them in one call. If the batch response is short or fails, items fall back to being classified one by one. A single-item call is retried once before falling back to storing unclassified.

## One gate before storage

Every result passes through one normalisation step before it is written. A result that looked usable can still be refused by storage (empty title, multi-level category); doing the repair in one place keeps that from losing a transcript. See [0003](../decisions/0003-never-drop-a-transcript.md).
