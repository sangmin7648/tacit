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

// Usable reports whether r carries enough content to store as a knowledge
// entry. A result that is not usable and not a skip means classification
// failed, not that the transcript was worthless.
func (r *ClassifyResult) Usable() bool {
	return r != nil && (r.Title != "" || r.Summary != "" || r.Category != "")
}

// Classifier is the strategy interface for LLM-based text classification.
type Classifier interface {
	Classify(ctx context.Context, sttText string, existingCategories []string) (*ClassifyResult, error)
	ClassifyBatch(ctx context.Context, texts []string, existingCategories []string) ([]*ClassifyResult, error)
}
