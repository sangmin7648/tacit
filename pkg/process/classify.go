package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClassifyResult holds the output from Claude Code CLI classification.
type ClassifyResult struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Category string `json:"category"`
}

const jsonSchema = `{"type":"object","properties":{"title":{"type":"string","description":"지식 제목 (한국어, 50자 이내)"},"summary":{"type":"string","description":"1-2문장 요약 (한국어)"},"category":{"type":"string","description":"카테고리 경로 (예: 개발/에러처리, 잡담). 최대 2단계."}},"required":["title","summary","category"]}`

const batchJsonSchema = `{"type":"object","properties":{"results":{"type":"array","items":{"type":"object","properties":{"title":{"type":"string","description":"지식 제목 (한국어, 50자 이내)"},"summary":{"type":"string","description":"1-2문장 요약 (한국어)"},"category":{"type":"string","description":"카테고리 경로 (예: 개발/에러처리, 잡담). 최대 2단계."}},"required":["title","summary","category"]}}},"required":["results"]}`

// Classify invokes Claude Code CLI to generate title, summary, and category
// for the given STT text. existingCategories provides context about current
// directory structure for consistent categorization.
func Classify(ctx context.Context, sttText string, existingCategories []string, model string) (*ClassifyResult, error) {
	if sttText == "" {
		return nil, fmt.Errorf("empty STT text")
	}

	if model == "" {
		model = "haiku"
	}

	prompt := buildPrompt(sttText, existingCategories)

	cmd := exec.CommandContext(ctx, "claude", "-p",
		"--output-format", "json",
		"--json-schema", jsonSchema,
		"--model", model,
		"--no-session-persistence",
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
		"--tools", "",
	)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude CLI failed: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("claude CLI failed: %w", err)
	}

	// Parse the JSON response from claude --output-format json
	// Format: {"result":"...", "structured_output": {"title":"...", "summary":"...", "category":"..."}, ...}
	var response struct {
		StructuredOutput *ClassifyResult `json:"structured_output"`
		Result           string          `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse claude output: %w, raw: %s", err, truncate(string(output), 200))
	}

	// Use structured_output if available (preferred)
	if response.StructuredOutput != nil {
		return response.StructuredOutput, nil
	}

	return nil, fmt.Errorf("no structured_output in claude response, raw result: %s", truncate(response.Result, 200))
}

// ClassifyBatch invokes Claude Code CLI once to classify multiple STT texts.
// This amortizes the CLI startup cost when multiple segments are queued.
func ClassifyBatch(ctx context.Context, texts []string, existingCategories []string, model string) ([]*ClassifyResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}
	if len(texts) == 1 {
		r, err := Classify(ctx, texts[0], existingCategories, model)
		if err != nil {
			return nil, err
		}
		return []*ClassifyResult{r}, nil
	}

	if model == "" {
		model = "haiku"
	}

	prompt := buildBatchPrompt(texts, existingCategories)

	cmd := exec.CommandContext(ctx, "claude", "-p",
		"--output-format", "json",
		"--json-schema", batchJsonSchema,
		"--model", model,
		"--no-session-persistence",
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
		"--tools", "",
	)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude CLI batch failed: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("claude CLI batch failed: %w", err)
	}

	var response struct {
		StructuredOutput *struct {
			Results []*ClassifyResult `json:"results"`
		} `json:"structured_output"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse batch output: %w, raw: %s", err, truncate(string(output), 200))
	}

	if response.StructuredOutput != nil && len(response.StructuredOutput.Results) > 0 {
		return response.StructuredOutput.Results, nil
	}

	return nil, fmt.Errorf("no structured_output in batch response, raw: %s", truncate(response.Result, 200))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildPrompt(sttText string, existingCategories []string) string {
	var sb strings.Builder
	sb.WriteString("STT 텍스트를 분석하여 제목, 요약, 카테고리를 생성하세요.\n")
	writeCategories(&sb, existingCategories)
	sb.WriteString("\nSTT 텍스트:\n")
	sb.WriteString(sttText)
	return sb.String()
}

func buildBatchPrompt(texts []string, existingCategories []string) string {
	var sb strings.Builder
	sb.WriteString("다음 STT 텍스트들을 각각 분석하여 제목, 요약, 카테고리를 생성하세요. 입력 순서대로 results 배열에 넣어주세요.\n")
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
