---
source: [tacit/pkg/storage]
verified: 9825e8e
---

# storage

Reads and writes entries and lists categories. The format and its rules are in [knowledge-entry](../concepts/knowledge-entry.md).

Writing validates the category and title and refuses path traversal; reading rejects files that are not valid entries. Also provides time-windowed listing, used by `tacit list` and the app.
