---
source: [pkg/search, pkg/storage, cmd/tacit]
verified: 9825e8e
---

# Searching the knowledge base

Three read operations exist: **list** entries from a time window, **search** by pattern, and **get** one entry in full. The CLI commands, the agent skills and the app's knowledge browser all use the same code, so they always find the same notes.

## Search is lexical

A pattern (a case-insensitive regular expression) is matched against the whole entry file. Results are ranked by a weighted count of matches: title counts most, then category, then summary, then the transcript body. Results can be limited to a recent window.

The engine is ripgrep, embedded in the binary and unpacked on first use, so the user installs nothing. Front-matter lines are not shown as matches, since the structured fields are already displayed.

## Why lexical

Entries are small and the classifier already writes recall-oriented keywords, so a regular-expression search over thousands of short files is fast and predictable, and needs no index to maintain or rebuild.
