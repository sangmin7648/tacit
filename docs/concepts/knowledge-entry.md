---
source: [tacit/core/internal/components/note-manager, tacit/core/internal/components/setting-manager]
verified: 9a2b7a0
---

# Knowledge entry and the knowledge base

The knowledge base is the directory `~/.tacit/` (or the configured base). Each entry is one Markdown file directly in that directory, named `<timestamp>.md`. Notes written before categories were removed sit in subdirectories and are still read; their `category` line is ignored.

## Shape of an entry

A front-matter block (title, creation time, keywords), then a one-sentence summary, a separator, and the raw transcript. Keywords are chosen for lexical recall: the search is text matching, not embeddings, so the classifier is asked for terms a future query might use.

## Rules storage enforces

- A title must be non-empty and at most 100 characters.
- File names are timestamps, so entries sort by time. If two entries start in the same second the later one gets a numeric suffix, so neither overwrites the other. The timestamp is when the entry's first speech began; later speech appended to the entry does not rename it.

An entry can hold several sessions of speech: the raw transcript section grows when the classifier says new speech continues the entry. See [classification](classification.md).

These rules are why classification output passes one normalising step before it is written. See [classification](classification.md).

## What else lives beside the entries

Under the same base: the settings files ([configuration](configuration.md)), downloaded Whisper models, the onboarding revision marker, the daemon's PID file and event log ([daemon-and-app](daemon-and-app.md), [events](events.md)).

## Why plain files

Users can read, edit, move, back up, and version them with ordinary tools, and agents can read them without a client library. See [0002](../decisions/0002-markdown-files-as-the-database.md).

## Reading

Listing, searching and fetching entries is shared by the CLI and the app. See [search](search.md).
