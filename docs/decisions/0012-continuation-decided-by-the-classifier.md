---
status: accepted
date: 2026-10-10
---

# Continuation is decided by the classifier

## Context

Every session became its own note. A pause of ten seconds, or the five-minute session cap, split one talk into several notes that each held a fragment and were hard to search as a whole.

## Decision

The classifier is shown the most recent note and says whether the new session continues it. If so, the transcript is appended to that note and the title and summary are rewritten to cover all of it. The previous note is offered only if it was last written within 30 minutes.

## Why

- Silence cannot tell a pause in a talk from a change of subject; the content can.
- A time gap alone would merge unrelated remarks made a minute apart.
- The session cap and segment splitting stay as they are: they bound the loss from one failed classification ([0003](0003-never-drop-a-transcript.md)), and a failed or unclassified note is never continued.
- The file is re-read before appending, so edits the user made by hand ([0002](0002-markdown-files-as-the-database.md)) are kept.

## Consequences

- The note keeps the timestamp and category of its first session.
- Only single classify calls carry the previous note; in a backlog batch only the first item is judged against it.
- A model that never answers "continues" degrades to the old behaviour, one note per session.
