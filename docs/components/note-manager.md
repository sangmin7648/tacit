---
source: [tacit/core/internal/components/note-manager]
verified: a6db5d7
---

# note-manager

Writes, reads, lists and searches the Markdown notes, and lists their categories. The format and its rules are in [knowledge-entry](../concepts/knowledge-entry.md); search is in [search](../concepts/search.md).

Writing validates the category and title and refuses path traversal; reading rejects files that are not valid entries. Search uses an embedded ripgrep, fetched at build time for macOS.
