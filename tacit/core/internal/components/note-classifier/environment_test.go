package noteclassifier

import "testing"

func TestHasOllamaModel(t *testing.T) {
	models := []string{"llama3.2:latest", "qwen3.5:8b"}
	for model, want := range map[string]bool{
		"llama3.2":   true,
		"qwen3.5:8b": true,
		"qwen3.5":    false,
		"mistral":    false,
	} {
		if got := HasOllamaModel(models, model); got != want {
			t.Errorf("HasOllamaModel(%q) = %v, want %v", model, got, want)
		}
	}
}
