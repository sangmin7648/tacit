# MCP Tool Contracts: tatic Knowledge Server

**Date**: 2026-03-28
**Transport**: stdio
**SDK**: `modelcontextprotocol/go-sdk` v1.0+
**Server Name**: `tatic`

## Server Capabilities

```json
{
  "name": "tatic",
  "version": "1.0.0",
  "capabilities": {
    "tools": {}
  }
}
```

---

## Tool 1: `search_knowledge`

키워드 매칭 기반 지식 검색. AI 에이전트가 검색 전략(키워드 변경, 범위 조정, 관련성 판단)을 수행하고, 이 tool은 원시 매칭 결과를 반환한다.

### Input Schema

```go
type SearchInput struct {
    Query           string `json:"query"             jsonschema:"required,description=Search keywords to match against title\\, summary\\, and content"`
    Category        string `json:"category,omitempty" jsonschema:"description=Category path filter (e.g. '개발' or '개발/에러처리'). When set to '잡담'\\, includes chitchat entries."`
    IncludeChitchat bool   `json:"include_chitchat,omitempty" jsonschema:"description=Include 잡담 (chitchat) category in results. Default false."`
}
```

```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Search keywords to match against title, summary, and content"
    },
    "category": {
      "type": "string",
      "description": "Category path filter (e.g. '개발' or '개발/에러처리'). When set to '잡담', includes chitchat entries."
    },
    "include_chitchat": {
      "type": "boolean",
      "description": "Include 잡담 (chitchat) category in results. Default false."
    }
  },
  "required": ["query"],
  "additionalProperties": false
}
```

### Output Schema

```json
{
  "type": "object",
  "properties": {
    "entries": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "description": "File path relative to knowledge base root" },
          "title": { "type": "string" },
          "category": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" },
          "summary": { "type": "string" },
          "match_context": { "type": "string", "description": "Snippet of matching text with surrounding context" }
        }
      }
    },
    "total": { "type": "integer" }
  }
}
```

### Behavior

- Searches `title`, `summary`, and `content` fields for keyword matches (case-insensitive)
- By default, excludes entries in `잡담/` category
- When `category` is specified, searches only within that category subtree
- When `category` is `잡담` or `include_chitchat` is `true`, includes chitchat entries
- Returns entries sorted by `created_at` descending (newest first)
- `match_context` contains ~100 characters around the first match

### Example

**Request**:
```json
{
  "query": "에러 핸들링",
  "category": "개발"
}
```

**Response**:
```json
{
  "entries": [
    {
      "id": "개발/에러처리/20260328-143052.md",
      "title": "Go 에러 핸들링에서 sentinel error 패턴 사용",
      "category": "개발/에러처리",
      "created_at": "2026-03-28T14:30:52+09:00",
      "summary": "Go에서 sentinel error를 정의하고 errors.Is로 비교하는 패턴에 대한 논의.",
      "match_context": "...Go에서 에러 핸들링할 때 sentinel error 패턴을 쓰면 좋은 게..."
    }
  ],
  "total": 1
}
```

---

## Tool 2: `list_knowledge`

지식 목록 조회. 카테고리 및 날짜 범위로 필터링 가능.

### Input Schema

```go
type ListInput struct {
    Category  string `json:"category,omitempty"   jsonschema:"description=Category path filter (e.g. '개발' or '개발/에러처리')"`
    DateRange string `json:"date_range,omitempty" jsonschema:"description=Date range filter in ISO format: 'YYYY-MM-DD..YYYY-MM-DD' (inclusive)"`
}
```

```json
{
  "type": "object",
  "properties": {
    "category": {
      "type": "string",
      "description": "Category path filter (e.g. '개발' or '개발/에러처리')"
    },
    "date_range": {
      "type": "string",
      "description": "Date range filter in ISO format: 'YYYY-MM-DD..YYYY-MM-DD' (inclusive)"
    }
  },
  "additionalProperties": false
}
```

### Output Schema

```json
{
  "type": "object",
  "properties": {
    "entries": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "description": "File path relative to knowledge base root" },
          "title": { "type": "string" },
          "category": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" },
          "summary": { "type": "string" }
        }
      }
    },
    "total": { "type": "integer" }
  }
}
```

### Behavior

- Lists knowledge entries matching the filters
- When no filters specified, lists all entries (excluding `잡담/` by default)
- `category` filter includes all subcategories (e.g., `개발` includes `개발/에러처리/`)
- `date_range` filters by `created_at` field (inclusive both ends)
- Returns entries sorted by `created_at` descending (newest first)

### Example

**Request**:
```json
{
  "category": "개발",
  "date_range": "2026-03-28..2026-03-28"
}
```

**Response**:
```json
{
  "entries": [
    {
      "id": "개발/에러처리/20260328-160230.md",
      "title": "gRPC 에러 코드 매핑 가이드",
      "category": "개발/에러처리",
      "created_at": "2026-03-28T16:02:30+09:00",
      "summary": "gRPC 상태 코드를 도메인 에러에 매핑하는 방법론 정리."
    },
    {
      "id": "개발/에러처리/20260328-143052.md",
      "title": "Go 에러 핸들링에서 sentinel error 패턴 사용",
      "category": "개발/에러처리",
      "created_at": "2026-03-28T14:30:52+09:00",
      "summary": "Go에서 sentinel error를 정의하고 errors.Is로 비교하는 패턴에 대한 논의."
    }
  ],
  "total": 2
}
```

---

## Tool 3: `get_knowledge`

특정 지식 항목의 전체 내용 조회.

### Input Schema

```go
type GetInput struct {
    ID string `json:"id" jsonschema:"required,description=Knowledge entry ID (file path relative to knowledge base root\\, e.g. '개발/에러처리/20260328-143052.md')"`
}
```

```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string",
      "description": "Knowledge entry ID (file path relative to knowledge base root, e.g. '개발/에러처리/20260328-143052.md')"
    }
  },
  "required": ["id"],
  "additionalProperties": false
}
```

### Output Schema

```json
{
  "type": "object",
  "properties": {
    "id": { "type": "string" },
    "title": { "type": "string" },
    "category": { "type": "string" },
    "created_at": { "type": "string", "format": "date-time" },
    "summary": { "type": "string" },
    "content": { "type": "string", "description": "Full STT transcript text" }
  }
}
```

### Behavior

- Returns the complete knowledge entry including full content
- Returns `isError: true` with message if the entry does not exist
- Path traversal (e.g., `../../etc/passwd`) is rejected with error

### Example

**Request**:
```json
{
  "id": "개발/에러처리/20260328-143052.md"
}
```

**Response**:
```json
{
  "id": "개발/에러처리/20260328-143052.md",
  "title": "Go 에러 핸들링에서 sentinel error 패턴 사용",
  "category": "개발/에러처리",
  "created_at": "2026-03-28T14:30:52+09:00",
  "summary": "Go에서 sentinel error를 정의하고 errors.Is로 비교하는 패턴에 대한 논의. 커스텀 에러 타입보다 간단하고 테스트하기 쉽다.",
  "content": "어 그래서 Go에서 에러 핸들링할 때 sentinel error 패턴을 쓰면 좋은 게 뭐냐면 errors.Is로 비교할 수 있잖아. 커스텀 에러 타입을 만드는 것보다 훨씬 간단하고 테스트도 쉬워. 특히 패키지 수준에서 var ErrNotFound = errors.New(\"not found\") 이렇게 정의해놓으면 호출하는 쪽에서 깔끔하게 처리할 수 있거든."
}
```

---

## Error Handling

All tools follow MCP error conventions:

- **Tool execution errors** (no results, invalid input): Return `CallToolResult` with `isError: true` and descriptive message text
- **Protocol errors** (malformed request, unknown tool): JSON-RPC error response (handled by SDK)
- **Path traversal attempts**: Rejected with `isError: true` and "invalid path" message

## Client Registration

### Claude Code (project scope)

`.mcp.json` at repository root:
```json
{
  "mcpServers": {
    "tatic": {
      "command": "tatic",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

### Claude Code (user scope)

```bash
claude mcp add --transport stdio tatic --scope user -- tatic mcp
```

> **Note**: `tatic` is a single binary. The `mcp` subcommand starts the MCP server in stdio mode.

### Claude Desktop

`~/Library/Application Support/Claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "tatic": {
      "command": "tatic",
      "args": ["mcp"]
    }
  }
}
```
