package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const singleSystemPrompt = `You classify speech-to-text transcripts into structured data.

SKIP CONDITION: Text with ONLY filler sounds (음, 어, 그, 아, 응, um, uh...) and no real words → set skip=true and leave other fields empty.

NORMAL CLASSIFICATION:
- title: specific topic of what was discussed
- summary: exactly ONE sentence condensing the key point (do NOT copy the input text)
- category: single Korean word/phrase describing the topic (NO slash, NO sub-category).
  Key distinctions:
  - coding/debugging/tech → 개발
  - algorithm study → 학습  (NOT 개발)
  - sprint/release planning → 업무  (NOT 개발)
  - cooking recipes → 생활  (NOT 일상)
  - personal feelings/reflection → 일기  (NOT 업무)
  - exercise/sports → 건강
  - For any other topic: invent a fitting single-word category
- keywords: array of 5–10 strings to maximize lexical search recall.
  Include: Korean synonyms, English equivalents, abbreviations, related concepts, and alternative phrasings.
  Cover both the topic AND its context (e.g. for a recipe: ingredient names, cooking methods, dish name variations).

EXAMPLES:
[1] "파이썬에서 데코레이터로 함수 실행 시간 측정하는 법 공부했어. functools.wraps 꼭 써야 원본 함수 이름 유지됨"
→ {"title":"파이썬 데코레이터 활용","summary":"파이썬 데코레이터로 실행 시간을 측정할 때 functools.wraps를 사용해야 원본 함수 정보가 유지된다","category":"개발","keywords":["데코레이터","decorator","functools.wraps","실행시간 측정","profiling","파이썬","python","함수 래퍼","wrapper","메타데이터 보존"]}

[2] "BFS랑 DFS 차이 드디어 이해했어. BFS는 너비 우선이라 최단경로에 쓰고 DFS는 깊이 우선이라 그래프 탐색이나 백트래킹에 씀"
→ {"title":"BFS와 DFS 차이 이해","summary":"BFS는 최단경로 탐색에, DFS는 그래프 탐색과 백트래킹에 적합하다는 것을 이해했다","category":"학습","keywords":["BFS","DFS","너비 우선 탐색","깊이 우선 탐색","breadth first search","depth first search","최단경로","그래프 탐색","백트래킹","backtracking"]}

[3] "이번 릴리즈에서 로그인 모듈 리팩토링 끝내야 해. 내가 백엔드 맡고 디자인팀이랑 연동은 다음 주까지"
→ {"title":"로그인 모듈 리팩토링 계획","summary":"이번 릴리즈에서 로그인 모듈 리팩토링을 완료하고 디자인팀과 다음 주까지 연동할 계획이다","category":"업무","keywords":["로그인","login","인증","auth","리팩토링","refactoring","릴리즈","release","백엔드","backend"]}

[4] "어제 수영 1km 했는데 팔이 너무 아파. 자유형 자세가 아직 어색한 것 같음"
→ {"title":"수영 후 팔 근육통","summary":"수영 1km를 완료했지만 팔이 많이 아파서 자유형 자세를 교정할 필요를 느꼈다","category":"건강","keywords":["수영","swimming","자유형","freestyle","근육통","DOMS","팔 통증","자세 교정","유산소","운동"]}

[5] "된장찌개 끓일 때 멸치 육수를 기본으로 해야 맛이 깊어. 두부랑 호박 넣고 마지막에 청양고추 추가하면 됨"
→ {"title":"된장찌개 레시피","summary":"된장찌개를 맛있게 끓이려면 멸치 육수를 기본으로 하고 두부와 호박, 청양고추를 넣는다","category":"생활","keywords":["된장찌개","doenjang jjigae","된장","멸치 육수","두부","호박","청양고추","한국 요리","찌개","레시피"]}

[6] "오늘 면접에서 긴장을 너무 많이 했어. 준비는 충분히 했는데 막상 말이 잘 안 나오더라. 다음엔 잘 할 수 있겠지"
→ {"title":"면접 긴장으로 인한 아쉬움","summary":"충분히 준비했지만 면접에서 긴장하여 말을 제대로 못해 아쉬움을 느꼈다","category":"일기","keywords":["면접","interview","긴장","불안","anxiety","취업","job","자기반성","발표","말하기"]}

[7] "마트에서 장 보다가 지갑을 두고 나왔어. 다시 가서 찾았는데 다행히 있었음"
→ {"title":"마트 지갑 분실 해프닝","summary":"마트에서 지갑을 두고 나왔다가 다시 찾은 일상적인 해프닝이 있었다","category":"일상","keywords":["지갑","wallet","분실","마트","장보기","해프닝","일상","깜빡","물건 찾기"]}

[8] "어... 그... 음..."
→ {"skip":true}`

const batchSystemPrompt = `You classify multiple speech-to-text transcripts. Return a JSON object with a "results" array preserving input order.

Each entry has:
- title: descriptive phrase of what was discussed (a sentence fragment, NOT a category label)
- summary: one sentence condensing the key point (do NOT copy the input)
- category: single Korean word/phrase (NO slash) that best fits the content.
  For NEW topics, invent a fitting single-word category. Examples of the principle:
  coding content → 개발, exercise → 건강, cooking → 생활,
  reading/reflection → 학습 or 일기, work planning → 업무
- keywords: array of 5–10 strings for lexical search recall.
  Include Korean synonyms, English equivalents, abbreviations, related concepts, and alternative phrasings.

Set skip=true only for pure filler sounds with no meaningful content.

EXAMPLE — two VAD-split segments from the same conversation:
--- 텍스트 1 ---
오늘 요가 수업 처음 갔는데 생각보다 훨씬 힘들더라
--- 텍스트 2 ---
특히 코어 운동 부분이 힘들었는데, 강사님이 매일 10분씩만 해도 된다고 하셔서 집에서 꾸준히 해보려고
→ {"results":[{"title":"요가 수업 첫 경험","summary":"요가 수업에 처음 참여했는데 생각보다 훨씬 힘들었다","category":"건강","keywords":["요가","yoga","첫 경험","운동","exercise","힘들다","유연성","flexibility","수업","class"]},{"title":"홈 요가 루틴 결심","summary":"코어 운동이 힘들었지만 강사 조언에 따라 매일 10분씩 집에서 꾸준히 하기로 결심했다","category":"건강","keywords":["코어","core","홈트","home workout","요가","yoga","루틴","routine","습관","habit"]}]}`

const defaultModel = "haiku"

// ClaudeClassifier implements Classifier using Claude Code CLI.
type ClaudeClassifier struct {
	model string
}

// NewClaudeClassifier creates a ClaudeClassifier with the given model name.
// If model is empty, defaults to "haiku".
func NewClaudeClassifier(model string) *ClaudeClassifier {
	if model == "" {
		model = defaultModel
	}
	return &ClaudeClassifier{model: model}
}

func (c *ClaudeClassifier) Classify(ctx context.Context, sttText string, existingCategories []string) (*ClassifyResult, error) {
	if sttText == "" {
		return nil, fmt.Errorf("empty STT text")
	}

	output, err := runClaude(ctx, singleSystemPrompt, buildPrompt(sttText, existingCategories), c.model)
	if err != nil {
		return nil, err
	}

	var result ClassifyResult
	if err := parseJSONFromText(output, &result); err != nil {
		return nil, fmt.Errorf("parse claude output: %w, raw: %s", err, truncate(string(output), 200))
	}
	return &result, nil
}

func (c *ClaudeClassifier) ClassifyBatch(ctx context.Context, texts []string, existingCategories []string) ([]*ClassifyResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}
	if len(texts) == 1 {
		r, err := c.Classify(ctx, texts[0], existingCategories)
		if err != nil {
			return nil, err
		}
		return []*ClassifyResult{r}, nil
	}

	output, err := runClaude(ctx, batchSystemPrompt, buildBatchPrompt(texts, existingCategories), c.model)
	if err != nil {
		return nil, err
	}

	var response struct {
		Results []*ClassifyResult `json:"results"`
	}
	if err := parseJSONFromText(output, &response); err != nil {
		return nil, fmt.Errorf("parse batch output: %w, raw: %s", err, truncate(string(output), 200))
	}
	if len(response.Results) == 0 {
		return nil, fmt.Errorf("empty results in batch response, raw: %s", truncate(string(output), 200))
	}
	return response.Results, nil
}

func runClaude(ctx context.Context, systemPrompt, prompt, model string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p",
		"--output-format", "text",
		"--system-prompt", systemPrompt,
		"--model", model,
		"--no-session-persistence",
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
		"--tools", "",
		"--effort", "low",
		"--setting-sources", "user",
		"--disable-slash-commands",
	)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude CLI failed: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("claude CLI failed: %w", err)
	}
	return output, nil
}

// parseJSONFromText extracts and parses JSON from Claude text output,
// stripping markdown code fences if present.
func parseJSONFromText(data []byte, v interface{}) error {
	text := strings.TrimSpace(string(data))

	// Strip markdown code fences: ```json ... ``` or ``` ... ```
	if strings.HasPrefix(text, "```") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) == 2 {
			text = lines[1]
		}
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	return json.Unmarshal([]byte(text), v)
}

// sanitizeResult cleans up LLM output artifacts.
func sanitizeResult(r *ClassifyResult) {
	if r == nil || r.Skip {
		return
	}
	// If all content fields are empty, the model indicated noise — treat as skip.
	if r.Title == "" && r.Summary == "" && r.Category == "" {
		r.Skip = true
		return
	}
	// If the model still returned a slash despite instructions, keep only the first segment.
	if idx := strings.Index(r.Category, "/"); idx >= 0 {
		r.Category = r.Category[:idx]
	}
	// Strip leading 잡담/ prefix (already handled above, but keep as safety)
	r.Category = strings.TrimPrefix(r.Category, "잡담/")
	// If category is exactly "잡담", replace with 일상
	if r.Category == "잡담" {
		r.Category = "일상"
	}
	// Trim extra whitespace from all fields
	r.Title = strings.TrimSpace(r.Title)
	r.Summary = strings.TrimSpace(r.Summary)
	r.Category = strings.TrimSpace(r.Category)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildPrompt(sttText string, existingCategories []string) string {
	var sb strings.Builder
	writeCategories(&sb, existingCategories)
	sb.WriteString("\nSTT 텍스트:\n")
	sb.WriteString(sttText)
	return sb.String()
}

func buildBatchPrompt(texts []string, existingCategories []string) string {
	var sb strings.Builder
	writeCategories(&sb, existingCategories)
	for i, text := range texts {
		fmt.Fprintf(&sb, "\n--- 텍스트 %d ---\n%s\n", i+1, text)
	}
	return sb.String()
}

func writeCategories(sb *strings.Builder, existingCategories []string) {
	if len(existingCategories) > 0 {
		sb.WriteString("기존 카테고리: ")
		sb.WriteString(strings.Join(existingCategories, ", "))
		sb.WriteString("\n기존 카테고리 우선 사용.")
	}
}
