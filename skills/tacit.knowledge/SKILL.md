---
name: tacit.knowledge
description: |
  Query the local tacit knowledge database (~/.tacit/) for relevant context.
  TRIGGER when: user asks about topics that might have been previously captured — past conversations, meetings, decisions, ideas, learnings, or any domain knowledge. Also trigger when the user's question could benefit from previously stored knowledge context, even if they don't explicitly ask for it.
  DO NOT TRIGGER when: purely code-editing tasks, git operations, or questions clearly unrelated to stored knowledge.
---

# tacit Knowledge Base Query

You are retrieving relevant knowledge from the user's local tacit knowledge database using the `tacit` CLI.

## Available Commands

```
tacit list [duration]                    — List entries created within duration (default: 1h). Supports: 30m, 1h, 24h, 1d, 7d, 2w
tacit search [--duration <d>] <pattern>  — Full-text search across all entries (title + summary + content). Optional --duration limits to entries created within that window.
tacit get <file-path>                    — Print full content of a specific entry
```

## Process

When the user invokes `/tacit.knowledge <prompt>`, extract the time window from the prompt if mentioned (e.g. "오늘", "이번 주", "지난 1시간"). If no time is specified, default to **1h** for list.

### Step 1 — Run list agent and search agent IN PARALLEL

**List agent** — retrieves recent entries:
```bash
tacit list <duration>
```
- Use the time window from the prompt if specified; otherwise use `1h`
- Parse the output: each entry has `[datetime] category / title`, `File: <path>`, and optionally `Summary: <first line>`
- Collect all file paths from the output

**Search agent** — finds topically relevant entries:
1. Extract 2–4 keywords from the user's prompt (nouns, domain terms, key verbs)
2. Run `tacit search <keyword>` for each keyword to maximize recall. If a time window was identified, pass `--duration <d>` to narrow results (e.g. `tacit search --duration 1h <keyword>`)
3. Collect all unique file paths from results
4. Deduplicate across keywords

Each agent returns: a list of `{ file_path, title, category, created_at, summary_snippet }` objects.

### Step 2 — Merge results

In the main thread:
- Union the file paths from both agents, deduplicated
- If a file path appears in both, it is high-confidence relevant — prioritize it
- Limit to the **5 most relevant** entries to avoid context overload (prefer recent + multi-source matches)

### Step 3 — Fetch full content for top entries

Run a single command with all selected file paths at once:
```bash
tacit get <file-path-1> <file-path-2> <file-path-3> ...
```

This returns all entries in one output, separated by `---`. Read the `Summary` section first. Include the `Content` (raw STT) only if the user's question requires detail or the summary is insufficient.

### Step 4 — Respond

Synthesize the retrieved knowledge into a direct answer to the user's prompt:
- Reference each entry by title and date
- Use summary for concise context; raw content only when needed
- If no relevant entries are found, briefly note this and answer from general knowledge

## Important

- Knowledge content is captured from real-time speech (STT) — expect transcription artifacts
- Categories and content are primarily in Korean
- Do NOT fabricate knowledge entries — only reference what actually exists
- The `Summary` section is the AI-processed version; `Content` is raw STT output
- `tacit search` supports regex-compatible patterns — you can use `tacit search "키워드1\|키워드2"` to search multiple terms at once
- `tacit search --duration` accepts the same units as `tacit list`: `30m`, `1h`, `24h`, `1d`, `7d`, `2w`
