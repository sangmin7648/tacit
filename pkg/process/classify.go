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
		"--tools", "",
		"--no-session-persistence",
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildPrompt(sttText string, existingCategories []string) string {
	var sb strings.Builder
	sb.WriteString("다음은 음성 인식(STT)으로 변환된 텍스트입니다. 이 텍스트를 분석하여 지식 DB에 저장할 메타데이터를 생성해주세요.\n\n")

	if len(existingCategories) > 0 {
		sb.WriteString("현재 지식 DB에 존재하는 카테고리 목록:\n")
		for _, cat := range existingCategories {
			sb.WriteString("- ")
			sb.WriteString(cat)
			sb.WriteString("\n")
		}
		sb.WriteString("\n가능하면 기존 카테고리를 사용하고, 적절한 카테고리가 없으면 새로 만들어주세요.\n")
		sb.WriteString("업무와 무관한 일상 대화, 잡담은 '잡담' 카테고리로 분류해주세요.\n\n")
	} else {
		sb.WriteString("아직 카테고리가 없습니다. 적절한 카테고리를 새로 만들어주세요.\n")
		sb.WriteString("업무와 무관한 일상 대화, 잡담은 '잡담' 카테고리로 분류해주세요.\n\n")
	}

	sb.WriteString("STT 텍스트:\n")
	sb.WriteString(sttText)

	return sb.String()
}
