---
source: [tacit/pkg/storage, tacit/pkg/config]
verified: 9825e8e
---

# Knowledge entry and the knowledge base

The knowledge base is the directory `~/.tacit/` (or the configured base). Each entry is one Markdown file at `<category>/<timestamp>.md`.

## Shape of an entry

A front-matter block (title, category, creation time, keywords), then a one-sentence summary, a separator, and the raw transcript. Keywords are chosen for lexical recall: the search is text matching, not embeddings, so the classifier is asked for terms a future query might use.

## Rules storage enforces

- A category is a single directory level. Slashes and path traversal are refused; non-ASCII names such as Korean are fine.
- A title must be non-empty and at most 100 characters.
- File names are timestamps, so entries sort by time within a category.

These rules are why classification output passes one normalising step before it is written. See [classification](classification.md).

## What else lives beside the entries

Under the same base: the settings files ([configuration](configuration.md)), downloaded Whisper models, the daemon's PID file and event log ([daemon-and-app](daemon-and-app.md), [events](events.md)).

## Why plain files

Users can read, edit, move, back up, and version them with ordinary tools, and agents can read them without a client library. See [0002](../decisions/0002-markdown-files-as-the-database.md).

## Reading

Listing, searching and fetching entries is shared by the CLI and the app. See [search](search.md).
