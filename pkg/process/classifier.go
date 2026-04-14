package process

import "context"

// ClassifyResult holds classification output from an LLM.
type ClassifyResult struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Category string   `json:"category"`
	Keywords []string `json:"keywords,omitempty"`
	Skip     bool     `json:"skip,omitempty"`
}

// Classifier is the strategy interface for LLM-based text classification.
type Classifier interface {
	Classify(ctx context.Context, sttText string, existingCategories []string) (*ClassifyResult, error)
	ClassifyBatch(ctx context.Context, texts []string, existingCategories []string) ([]*ClassifyResult, error)
}
