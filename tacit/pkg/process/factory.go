package process

import (
	"context"

	"github.com/sangmin7648/tacit/pkg/config"
)

// Pinger is an optional interface for classifiers that support startup health checks.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewClassifier creates a Classifier based on the LLMProvider in cfg.
// Supported providers: "claude" (default), "ollama".
func NewClassifier(cfg *config.Config) Classifier {
	switch cfg.LLMProvider {
	case "ollama":
		return NewOllamaClassifier("", cfg.LLMModel)
	default:
		return NewClaudeClassifier(cfg.LLMModel)
	}
}
