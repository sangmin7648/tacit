---
name: tacit.knowledge
description: |
  Query the local tacit knowledge database (~/.tacit/) for relevant context.
  TRIGGER when: user asks about topics that might have been previously captured — past conversations, meetings, decisions, ideas, learnings, or any domain knowledge. Also trigger when the user's question could benefit from previously stored knowledge context, even if they don't explicitly ask for it.
  DO NOT TRIGGER when: purely code-editing tasks, git operations, or questions clearly unrelated to stored knowledge.
---

# tacit Knowledge Base Query

You are retrieving relevant knowledge from multiple sources: the local tacit knowledge database and any external knowledge MCPs available in the current session.

## Available tacit Commands

```
tacit list [duration]                    — List entries created within duration (default: 1h). Supports: 30m, 1h, 24h, 1d, 7d, 2w
tacit search [--duration <d>] <pattern>  — Full-text search across all entries (title + summary + content). Optional --duration limits to entries created within that window.
tacit get <file-path>                    — Print full content of a specific entry
```

## Process

When the user invokes `/tacit.knowledge <prompt>`, extract the time window from the prompt if mentioned (e.g. "오늘", "이번 주", "지난 1시간"). If no time is specified, default to **1h** for list.

### Step 0 — Identify available external knowledge sources

Review the tools available in the current session. Identify MCP tools that provide **search or query** capabilities over knowledge repositories — such as messaging platforms, wikis, note-taking apps, project management tools, email, calendars, or issue trackers.

Criteria for inclusion:
- The tool name or description contains keywords like `search`, `query`, `find`, `list`, `read`
- The tool belongs to a service where knowledge, decisions, or discussions are likely stored
- Do **not** hardcode specific tool names — evaluate based on what is actually available

### Step 1 — Launch sub-agents in parallel

Launch one sub-agent per source, all in parallel using the Agent tool. Each sub-agent is isolated and does not pollute the main context.

**tacit sub-agent** — retrieves from local knowledge base:
1. Run `tacit list <duration>` to get recent entries
2. Extract 2–4 keywords from the user's prompt and run `tacit search <keyword>` for each. If a time window was identified, pass `--duration <d>` to narrow results (e.g. `tacit search --duration 1h <keyword>`)
3. Merge all unique file paths, prioritizing entries that appear in both list and search results
4. Fetch full content: `tacit get <path1> <path2> ...`
5. Return structured results: `{ source: "tacit", items: [{ title, file_path, date, category, summary, content }] }`

**External source sub-agent(s)** — one per identified MCP source:
- Use the available search/query tools for that source
- Extract keywords from the user's prompt and run targeted queries
- Perform follow-up queries or pagination as needed to explore deeply
- Return structured results: `{ source: "<service name>", items: [{ title, url_or_ref, date, snippet }] }`
- If a tool is unavailable or returns an error, return an empty result silently

### Step 2 — Merge results in the main thread

Collect all sub-agent results and merge:
- Tag each item with its source (e.g. `tacit`, `Slack`, `Notion`, etc.)
- Items appearing in multiple sources are high-confidence — surface them first
- No item count limit — use everything the sub-agents returned

### Step 3 — Respond

Synthesize all retrieved knowledge into a direct answer to the user's prompt:
- Group or sequence results naturally (by topic, time, or source)
- For each item, cite: **title**, **date**, and **source** (channel, page, tacit category, etc.)
- Use summaries for concise context; include raw content only when the user's question requires detail
- If no relevant entries are found across any source, briefly note this and answer from general knowledge

## Important

- tacit content is captured from real-time speech (STT) — expect transcription artifacts
- tacit categories and content are primarily in Korean
- Do NOT fabricate knowledge entries — only reference what actually exists
- `tacit search` supports regex-compatible patterns — you can use `tacit search "키워드1\|키워드2"` to search multiple terms at once
- `tacit search --duration` accepts the same units as `tacit list`: `30m`, `1h`, `24h`, `1d`, `7d`, `2w`
- When launching external source sub-agents, pass the user's original prompt and extracted keywords so each sub-agent can independently determine the best query strategy for its source
