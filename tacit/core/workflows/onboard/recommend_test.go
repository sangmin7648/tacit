package onboard

import "testing"

func TestRecommendLanguage(t *testing.T) {
	cases := []struct {
		name      string
		preferred []string
		want      string
	}{
		{"one offered language", []string{"ko-KR"}, "ko"},
		{"same language twice", []string{"en-US", "en-GB"}, "en"},
		{"two offered languages", []string{"en-KR", "ko-KR"}, "auto"},
		{"none offered", []string{"fr-FR"}, "auto"},
		{"unreadable", nil, "auto"},
		{"other languages ignored", []string{"fr-FR", "ko-KR"}, "ko"},
	}
	for _, c := range cases {
		r := &Recommendation{Reasons: map[string]string{}}
		r.recommendLanguage(c.preferred)
		if r.Choices.Language != c.want {
			t.Errorf("%s: got %q, want %q", c.name, r.Choices.Language, c.want)
		}
		if r.Reasons[ReasonLanguage] == "" {
			t.Errorf("%s: no reason given", c.name)
		}
	}
}

func TestRecommendClassifier_AlwaysLocal(t *testing.T) {
	cases := []struct {
		name   string
		claude bool
		ollama OllamaStatus
	}{
		{"ollama running", true, OllamaStatus{Installed: true, Running: true}},
		{"only claude", true, OllamaStatus{}},
		{"ollama installed, stopped", false, OllamaStatus{Installed: true}},
		{"nothing", false, OllamaStatus{}},
	}
	for _, c := range cases {
		r := &Recommendation{Reasons: map[string]string{}, ClaudeAvailable: c.claude, Ollama: c.ollama}
		r.recommendClassifier(32 << 30)
		if r.Choices.LLMProvider != "ollama" || r.Choices.LLMModel != DefaultOllamaModel {
			t.Errorf("%s: recommends %s/%s, want the local model", c.name, r.Choices.LLMProvider, r.Choices.LLMModel)
		}
		if r.Reasons[ReasonProvider] == "" || r.Reasons[ReasonModel] == "" {
			t.Errorf("%s: missing reasons %v", c.name, r.Reasons)
		}
		if r.MemoryWarning != "" {
			t.Errorf("%s: warned about memory on a 32 GB Mac: %s", c.name, r.MemoryWarning)
		}
	}
}

func TestRecommendClassifier_WarnsWhenMemoryIsShort(t *testing.T) {
	for _, c := range []struct {
		gb      int64
		warning bool
	}{{8, true}, {15, true}, {16, false}, {0, false}} {
		r := &Recommendation{Reasons: map[string]string{}, MemoryGB: int(c.gb)}
		r.recommendClassifier(c.gb << 30)
		if (r.MemoryWarning != "") != c.warning {
			t.Errorf("%d GB: warning %q, want warning=%v", c.gb, r.MemoryWarning, c.warning)
		}
		if r.Choices.LLMProvider != "ollama" {
			t.Errorf("%d GB: recommends %s; a short Mac is warned, not sent elsewhere", c.gb, r.Choices.LLMProvider)
		}
	}
}

func TestRecommendWhisper_NamesTheMemory(t *testing.T) {
	isolate(t)
	r := &Recommendation{Reasons: map[string]string{}, MemoryGB: 32}
	r.recommendWhisper(32 << 30)
	if r.Choices.WhisperModel != "large-v3-turbo" {
		t.Fatalf("got %q on 32 GB", r.Choices.WhisperModel)
	}
	rec := 0
	for _, m := range r.WhisperModels {
		if m.Recommended {
			rec++
		}
	}
	if rec != 1 {
		t.Errorf("%d models marked recommended, want 1", rec)
	}
	if r.Reasons[ReasonWhisper] == "" {
		t.Error("no reason given")
	}
}

func TestRecommend_ProducesValidChoices(t *testing.T) {
	isolate(t)
	if err := Recommend(t.Context()).Choices.Validate(); err != nil {
		t.Errorf("recommended choices are invalid: %v", err)
	}
}
