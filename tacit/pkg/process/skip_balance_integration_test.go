//go:build integration

package process

import (
	"context"
	"testing"
	"time"
)

// TestClassifier_SkipBalance_Ollama runs the shared incident corpus (see
// corpus_test.go, which pins the model-free half of the same contract) against
// a live model. It exercises the ollama path specifically: the schema that
// decides this only applies there, and the other integration test runs against
// the Claude CLI.
func TestClassifier_SkipBalance_Ollama(t *testing.T) {
	o := NewOllamaClassifier("", "qwen3.5")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := o.Ping(ctx); err != nil {
		t.Skipf("ollama unavailable: %v", err)
	}

	categories := []string{"dev", "work", "daily"}

	for _, c := range corpusKeep {
		// The pipeline strips hallucinated boilerplate before classifying, so
		// the classifier sees what it would see in production.
		cleaned := FilterHallucinations(c.text, nil)
		r, err := o.Classify(ctx, cleaned, categories)
		if err != nil {
			t.Fatalf("classify %q: %v", cleaned, err)
		}
		final := Finalize(r, cleaned)
		switch {
		case r.Skip:
			t.Errorf("real speech was skipped: %q", c.text)
		case final.Title == "" || final.Category == "":
			t.Errorf("result is not storable for %q: title=%q category=%q", c.text, final.Title, final.Category)
		}
	}

	for _, text := range corpusDrop {
		if IsFiller(text) {
			continue // the Go gate already catches it; the model is never asked
		}
		r, err := o.Classify(ctx, text, categories)
		if err != nil {
			t.Fatalf("classify %q: %v", text, err)
		}
		if !r.Skip {
			t.Errorf("worthless transcript was kept: %q -> title=%q category=%q", text, r.Title, r.Category)
		}
	}
}
