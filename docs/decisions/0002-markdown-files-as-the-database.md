---
status: accepted
date: 2026-03-28
source: [tacit/core/internal/components/note-manager]
verified: a6db5d7
---

# 0002. Markdown files are the database

**Decision.** Each entry is a Markdown file with front matter in a category directory. There is no database, index, or server.

**Why.** The consumers are people and AI agents. Both read files natively; users can edit, move, back up and version them, and the data outlives the tool. A database would need a client, a schema migration story, and a way to repair corruption.

**Consequences.**
- Search is a text scan, not an index. That is fast enough at personal scale and has nothing to rebuild. See [search](../concepts/search.md).
- Structure is enforced at write time, because nothing else enforces it. See [knowledge-entry](../concepts/knowledge-entry.md).
- Users may hand-edit entries, so readers must tolerate imperfect files.
