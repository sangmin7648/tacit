package process

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const defaultOllamaBaseURL = "http://localhost:11434"
const defaultOllamaModel = "llama3.2"

// OllamaClassifier implements Classifier using the Ollama HTTP API.
type OllamaClassifier struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaClassifier creates an OllamaClassifier.
// If baseURL is empty, defaults to http://localhost:11434.
// If model is empty, defaults to llama3.2.
func NewOllamaClassifier(baseURL, model string) *OllamaClassifier {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	if model == "" {
		model = defaultOllamaModel
	}
	return &OllamaClassifier{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

// Ping checks that the Ollama server is reachable and the configured model exists.
func (o *OllamaClassifier) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama server not reachable at %s\n  → Is Ollama running? Try: ollama serve", o.baseURL)
	}
	defer resp.Body.Close()

	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode tags response: %w", err)
	}

	for _, m := range body.Models {
		// model names can be "llama3.2" or "llama3.2:latest"
		name := strings.TrimSuffix(m.Name, ":latest")
		if name == o.model || m.Name == o.model {
			return nil
		}
	}
	return fmt.Errorf("Ollama model %q not found\n  → Pull it with: ollama pull %s", o.model, o.model)
}

func (o *OllamaClassifier) Classify(ctx context.Context, sttText string, existingCategories []string) (*ClassifyResult, error) {
	if sttText == "" {
		return nil, fmt.Errorf("empty STT text")
	}

	output, err := o.runOllama(ctx, singleSystemPrompt, buildPrompt(sttText, existingCategories))
	if err != nil {
		return nil, err
	}

	var result ClassifyResult
	if err := parseJSONFromText([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("parse ollama output: %w, raw: %s", err, truncate(output, 200))
	}
	return &result, nil
}

func (o *OllamaClassifier) ClassifyBatch(ctx context.Context, texts []string, existingCategories []string) ([]*ClassifyResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}
	if len(texts) == 1 {
		r, err := o.Classify(ctx, texts[0], existingCategories)
		if err != nil {
			return nil, err
		}
		return []*ClassifyResult{r}, nil
	}

	output, err := o.runOllama(ctx, batchSystemPrompt, buildBatchPrompt(texts, existingCategories))
	if err != nil {
		return nil, err
	}

	var response struct {
		Results []*ClassifyResult `json:"results"`
	}
	if err := parseJSONFromText([]byte(output), &response); err != nil {
		return nil, fmt.Errorf("parse batch output: %w, raw: %s", err, truncate(output, 200))
	}
	if len(response.Results) == 0 {
		return nil, fmt.Errorf("empty results in batch response, raw: %s", truncate(output, 200))
	}
	return response.Results, nil
}

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	System string `json:"system"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

func (o *OllamaClassifier) runOllama(ctx context.Context, systemPrompt, prompt string) (string, error) {
	reqBody := ollamaGenerateRequest{
		Model:  o.model,
		System: systemPrompt,
		Prompt: prompt,
		Stream: false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf strings.Builder
		json.NewDecoder(resp.Body).Decode(&buf)
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}

	return result.Response, nil
}
