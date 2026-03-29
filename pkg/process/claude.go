package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const singleSystemPrompt = `You are a speech-to-text classifier. Output only valid JSON, no extra text.
Format: {"title":"brief topic of what was said (not the category)","summary":"one sentence summary of the content","category":"category(max 2 levels)"}
Example: input="오늘 회의에서 일정 조율했어" → {"title":"일정 조율","summary":"오늘 회의에서 일정을 조율했다","category":"업무/회의"}
If the input is meaningless noise or unclear, output only: {"skip":true}
All JSON values must be in the same language as the input text.`

const batchSystemPrompt = `You are a speech-to-text classifier. Output only valid JSON, no extra text.
Format: {"results":[{"title":"brief topic of what was said (not the category)","summary":"one sentence summary of the content","category":"category(max 2 levels)"}]}
Preserve input order. If an entry is meaningless noise or unclear, use {"skip":true} for that entry.
All JSON values must be in the same language as the input text.`

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
		sb.WriteString("\n기존 카테고리 우선 사용. 일상/잡담→'잡담'.")
	} else {
		sb.WriteString("카테고리 새로 생성. 일상/잡담→'잡담'.")
	}
}
