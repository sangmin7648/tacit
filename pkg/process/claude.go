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
- title: specific topic of what was discussed — write in the same language as the input
- summary: exactly ONE sentence condensing the key point (do NOT copy the input text) — write in the same language as the input
- category: concise English word or short phrase describing the topic (NO slash, NO sub-category). Always English regardless of input language.
  Key distinctions:
  - coding/debugging/tech → dev
  - algorithm study → learning  (NOT dev)
  - sprint/release planning → work  (NOT dev)
  - cooking recipes → lifestyle  (NOT daily)
  - personal feelings/reflection → journal  (NOT work)
  - exercise/sports → health
  - For any other topic: invent a fitting single English word
- keywords: array of 5–10 strings to maximize lexical search recall.
  Include synonyms, abbreviations, related concepts, and alternative phrasings — use the input language plus English equivalents.
  Cover both the topic AND its context (e.g. for a recipe: ingredient names, cooking methods, dish name variations).

EXAMPLES:
[1] "I studied how to measure function execution time with Python decorators. You have to use functools.wraps to preserve the original function name."
→ {"title":"Python decorator for execution timing","summary":"When measuring execution time with Python decorators, functools.wraps must be used to preserve the original function metadata","category":"dev","keywords":["decorator","functools.wraps","execution time","profiling","python","function wrapper","wrapper","metadata","timing","performance"]}

[2] "I finally understood the difference between BFS and DFS. BFS is breadth-first so it's used for shortest paths, DFS is depth-first for graph traversal and backtracking."
→ {"title":"Understanding BFS vs DFS","summary":"BFS is suited for shortest path search while DFS is suited for graph traversal and backtracking","category":"learning","keywords":["BFS","DFS","breadth first search","depth first search","shortest path","graph traversal","backtracking","algorithm","tree","search"]}

[3] "We need to finish refactoring the login module in this release. I'm handling the backend and the design team integration is due next week."
→ {"title":"Login module refactoring plan","summary":"The login module refactoring must be completed this release with backend done by me and design team integration due next week","category":"work","keywords":["login","auth","authentication","refactoring","release","backend","integration","sprint","module","deadline"]}

[4] "My arms are so sore after swimming 1km yesterday. My freestyle form still feels off."
→ {"title":"Arm soreness after swimming 1km","summary":"Completed a 1km swim but arms are very sore, indicating a need to correct freestyle form","category":"health","keywords":["swimming","freestyle","muscle soreness","DOMS","arm pain","form","aerobic","exercise","1km","technique"]}

[5] "For a good miso soup you need anchovy broth as the base. Add tofu and zucchini, then throw in some chili pepper at the end."
→ {"title":"Miso soup recipe","summary":"A good miso soup uses anchovy broth as the base with tofu, zucchini, and chili pepper added at the end","category":"lifestyle","keywords":["miso soup","doenjang jjigae","anchovy broth","tofu","zucchini","chili pepper","Korean soup","recipe","stew","cooking"]}

[6] "I was so nervous during the interview today. I was prepared but when the moment came I couldn't get the words out. I'll do better next time."
→ {"title":"Regret over interview nerves","summary":"Despite adequate preparation, nerves during the interview prevented clear communication, prompting self-reflection","category":"journal","keywords":["interview","nervousness","anxiety","job","self-reflection","presentation","speaking","preparation","regret","growth"]}

[7] "I left my wallet at the grocery store while shopping. Went back and thankfully it was still there."
→ {"title":"Wallet left at grocery store","summary":"Left my wallet at the grocery store while shopping but was able to retrieve it","category":"daily","keywords":["wallet","lost","grocery","shopping","everyday","forgetful","found","errand","incident"]}

[8] "파이썬에서 데코레이터로 함수 실행 시간 측정하는 법 공부했어. functools.wraps 꼭 써야 원본 함수 이름 유지됨"
→ {"title":"파이썬 데코레이터 활용","summary":"파이썬 데코레이터로 실행 시간을 측정할 때 functools.wraps를 사용해야 원본 함수 정보가 유지된다","category":"dev","keywords":["데코레이터","decorator","functools.wraps","실행시간 측정","profiling","파이썬","python","함수 래퍼","wrapper","메타데이터 보존"]}

[9] "uh... so... um..."
→ {"skip":true}`

const batchSystemPrompt = `You classify multiple speech-to-text transcripts. Return a JSON object with a "results" array preserving input order.

Each entry has:
- title: descriptive phrase of what was discussed (a sentence fragment, NOT a category label) — write in the same language as the input
- summary: one sentence condensing the key point (do NOT copy the input) — write in the same language as the input
- category: concise English word or short phrase (NO slash) that best fits the content. Always English regardless of input language.
  For NEW topics, invent a fitting single English word. Examples of the principle:
  coding content → dev, exercise → health, cooking → lifestyle,
  reading/reflection → learning or journal, work planning → work
- keywords: array of 5–10 strings for lexical search recall.
  Include synonyms, abbreviations, related concepts, and alternative phrasings — use the input language plus English equivalents.

Set skip=true only for pure filler sounds with no meaningful content.

EXAMPLE — two VAD-split segments from the same conversation:
--- text 1 ---
I went to my first yoga class today and it was way harder than I expected
--- text 2 ---
The core section was especially tough, but the instructor said even 10 minutes a day is enough, so I want to keep it up at home
→ {"results":[{"title":"First yoga class experience","summary":"Attended yoga for the first time and found it much harder than expected","category":"health","keywords":["yoga","first time","exercise","workout","class","flexibility","beginner","fitness","stretching","studio"]},{"title":"Commitment to home yoga routine","summary":"Despite the difficulty of the core section, decided to practice 10 minutes daily at home following the instructor's advice","category":"health","keywords":["core","home workout","yoga","routine","habit","daily","10 minutes","consistency","practice","fitness"]}]}`

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
	// Strip leading chat/ prefix (already handled above, but keep as safety)
	r.Category = strings.TrimPrefix(r.Category, "chat/")
	// If category is exactly "chat", replace with daily
	if r.Category == "chat" {
		r.Category = "daily"
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
	sb.WriteString("\nSTT text:\n")
	sb.WriteString(sttText)
	return sb.String()
}

func buildBatchPrompt(texts []string, existingCategories []string) string {
	var sb strings.Builder
	writeCategories(&sb, existingCategories)
	for i, text := range texts {
		fmt.Fprintf(&sb, "\n--- text %d ---\n%s\n", i+1, text)
	}
	return sb.String()
}

func writeCategories(sb *strings.Builder, existingCategories []string) {
	if len(existingCategories) > 0 {
		sb.WriteString("Existing categories: ")
		sb.WriteString(strings.Join(existingCategories, ", "))
		sb.WriteString("\nPrefer existing categories when they fit.")
	}
}
