package noteclassifier

import (
	"context"
	"time"
)

// ClassifyResult holds classification output from an LLM.
type ClassifyResult struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Category string   `json:"category"`
	Keywords []string `json:"keywords,omitempty"`
	Skip     bool     `json:"skip,omitempty"`
	// Continues is the model's verdict that the text carries on the PreviousNote
	// it was shown. It is only honoured when a previous note was offered.
	Continues bool `json:"continues,omitempty"`
}

// PreviousNote is the most recently stored note, offered to Classify so the
// model can say whether new speech belongs in it instead of starting another.
type PreviousNote struct {
	Title   string
	Summary string
	Ago     time.Duration // since the note was last written
}

// Usable reports whether r carries enough content to store as a knowledge
// entry. A result that is not usable and not a skip means classification
// failed, not that the transcript was worthless.
func (r *ClassifyResult) Usable() bool {
	return r != nil && (r.Title != "" || r.Summary != "" || r.Category != "")
}

// Classifier is the strategy interface for LLM-based text classification.
type Classifier interface {
	Classify(ctx context.Context, sttText string, existingCategories []string, previous *PreviousNote) (*ClassifyResult, error)
	ClassifyBatch(ctx context.Context, texts []string, existingCategories []string) ([]*ClassifyResult, error)
}
