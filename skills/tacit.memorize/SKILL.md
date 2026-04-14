---
name: tacit.memorize
description: |
  Summarize the current session and save it to the tacit knowledge base (~/.tacit/memory/).
  TRIGGER when: the user invokes /tacit.memorize, wants to save the current conversation to memory, or asks to record what was discussed.
  DO NOT TRIGGER for: querying existing knowledge (use /tacit.knowledge instead).
---

# tacit Memorize — Session → Knowledge Entry

Save the current conversation as a structured knowledge entry to `~/.tacit/memory/<category>/YYYYMMDD-HHMMSS.md`.

## Process

### Step 1 — Analyze the conversation

Read the current conversation and extract:
- **title**: A concise title (≤ 100 chars) capturing the main topic — write in the conversation's language
- **category**: A single English word describing the domain (e.g. `dev`, `meeting`, `learning`, `design`, `ideas`, `debugging`) — always English
- **keywords**: 5–10 search terms to maximize lexical recall in `tacit search`. Include:
  - Synonyms and related concepts in the conversation's language plus English equivalents
  - Abbreviations and alternative phrasings someone might search for
  - Specific names: tools, methods, people, places mentioned
- **summary**: Bullet list of key insights, decisions, and learnings — write in the conversation's language
- **content**: Detailed notes — background, context, rationale, specifics — write in the conversation's language

If the user provided a hint with the command (e.g. `/tacit.memorize skill development`), use it to guide the title and category.

### Step 2 — Determine the file path

Run the following to get a timestamp:

```bash
date +%Y%m%d-%H%M%S
```

Also get the current timezone offset for `created_at`:

```bash
date +%z
```

Build the paths:
- `CATEGORY_DIR` = `~/.tacit/memory/<category>`
- `FILE_PATH` = `~/.tacit/memory/<category>/YYYYMMDD-HHMMSS.md`

Create the directory:

```bash
mkdir -p ~/.tacit/memory/<category>
```

### Step 3 — Write the file

Write the file using the Write tool at the exact path from Step 2. Use this format:

```
---
title: "<title>"
category: "<category>"
created_at: "<YYYY-MM-DDTHH:MM:SS+HH:MM>"
keywords: ["<keyword1>", "<keyword2>", ...]
---

<summary — bullet list of key points>

---

<content — detailed notes, background, context>
```

**Format rules:**
- `title` and `category` must be quoted strings in the YAML frontmatter
- `created_at` must be ISO 8601 with timezone offset (e.g. `"2026-04-02T15:04:05+09:00"`)
- `keywords` must be a JSON-style inline array of quoted strings on one line
- Summary and content are plain Markdown (no YAML)
- Separate frontmatter from summary with `---\n\n` (blank line after)
- Separate summary from content with `\n\n---\n\n` (blank lines around)

### Step 4 — Report

Print the saved file path so the user can verify:

```
Saved to: ~/.tacit/memory/<category>/YYYYMMDD-HHMMSS.md
```

## Important

- Write to the **absolute path** (expand `~` to the actual home directory path)
- Do NOT fabricate or embellish — only record what actually appeared in the conversation
- Summary = concise takeaways; Content = full detail
- Categories are single-level, no slashes (e.g. `dev` not `dev/backend`)
- The entry will automatically appear in `tacit list` and `tacit search` — no additional steps needed
