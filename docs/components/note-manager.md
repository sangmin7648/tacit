---
source: [tacit/core/internal/components/note-manager]
verified: 9a2b7a0
---

# note-manager

Writes, reads, lists and searches the Markdown notes. The format and its rules are in [knowledge-entry](../concepts/knowledge-entry.md); search is in [search](../concepts/search.md).

Writing validates the title and never overwrites an existing note; reading rejects files that are not valid entries. Search uses an embedded ripgrep, fetched at build time for macOS.
