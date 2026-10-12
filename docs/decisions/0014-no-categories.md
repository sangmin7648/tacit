---
status: accepted
date: 2026-10-12
---

# Notes have no categories

## Context

The classifier named a category for every note and the note was stored in a directory of that name. The names came from a model choosing a word per transcript, so the set drifted: near-duplicates, one-off words, and the same kind of talk filed under different names on different days. Nobody browsed by category, and the grouping misled more than it helped.

## Decision

A note has a title, keywords, a summary and a transcript, and no category. The classifier is not asked for one and is no longer shown the existing ones. Notes are stored directly in the knowledge base, named by timestamp. This supersedes the category directory in [0002](0002-markdown-files-as-the-database.md) and the `unsorted` category in [0003](0003-never-drop-a-transcript.md); the rest of both still holds.

Notes already on disk are left where they are. They are read from any depth, and their `category` front-matter line is ignored.

## Why

- Keywords and search already find notes by topic; a second, unstable label added noise.
- Without a directory per category, a flat folder makes two notes starting in the same second collide, so storage now adds a suffix instead of overwriting, which would have lost a transcript.

## Consequences

- `tacit list|search|get --json` no longer print `category`; the JSON version is 2.
- Agents that read the folder find notes in one place instead of walking category directories.
