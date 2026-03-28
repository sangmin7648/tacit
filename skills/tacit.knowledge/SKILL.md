---
name: tacit.knowledge
description: |
  Query the local tacit knowledge database (~/.tacit/) for relevant context.
  TRIGGER when: user asks about topics that might have been previously captured — past conversations, meetings, decisions, ideas, learnings, or any domain knowledge. Also trigger when the user's question could benefit from previously stored knowledge context, even if they don't explicitly ask for it.
  DO NOT TRIGGER when: purely code-editing tasks, git operations, or questions clearly unrelated to stored knowledge.
---

# tacit Knowledge Base Query

You are retrieving relevant knowledge from the user's local tacit knowledge database at `~/.tacit/`.

## Knowledge File Format

Each file is Markdown with YAML frontmatter:
```
---
title: "제목"
category: "카테고리/서브카테고리"
created_at: "2006-01-02T15:04:05-07:00"
---

요약 내용

---

원본 전사 내용
```

Files are stored at: `~/.tacit/<category>/<subcategory>/YYYYMMDD-HHMMSS.md`

## Process

1. **List categories** to understand what knowledge exists:
   ```
   ls ~/.tacit/ (top-level categories)
   ls ~/.tacit/<category>/ (subcategories and files)
   ```

2. **Identify relevant categories** based on the user's question or current conversation context.

3. **List files** in relevant categories (sorted by date, most recent first):
   ```
   ls -t ~/.tacit/<category>/
   ls -t ~/.tacit/<category>/<subcategory>/
   ```

4. **Search for keywords** if the topic is specific:
   - Use grep across knowledge files to find relevant entries
   - Search in both title (frontmatter) and summary sections

5. **Read relevant files** — prioritize:
   - Most recent entries first
   - Files whose title/summary matches the query topic
   - Limit to 3-5 most relevant entries to avoid context overload

6. **Present the knowledge** naturally integrated into your response:
   - Reference the title and date of each knowledge entry
   - Use the summary for concise context; include content (raw transcript) only if the user needs details
   - If no relevant knowledge is found, briefly mention that and continue answering normally

## Important

- Knowledge is captured from real-time speech (STT), so content may have transcription artifacts
- Categories and content are primarily in Korean
- Do NOT fabricate knowledge entries — only reference what actually exists in the files
- The summary section is the classified/processed version; content section is raw STT output
