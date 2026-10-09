package noteclassifier

import (
	"context"

	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
)

// Pinger is an optional interface for classifiers that support startup health checks.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewClassifier creates a Classifier based on the LLMProvider in cfg.
// Supported providers: "claude" (default), "ollama".
func NewClassifier(cfg *settingmanager.Config) Classifier {
	switch cfg.LLMProvider {
	case "ollama":
		return NewOllamaClassifier("", cfg.LLMModel)
	default:
		return NewClaudeClassifier(cfg.LLMModel)
	}
}
