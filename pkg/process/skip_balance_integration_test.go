//go:build integration

package process

import (
	"context"
	"testing"
	"time"
)

// Skip has failed in both directions. Before #12 the classifier discarded real
// speech; the first fix made the content fields required, which stopped the
// loss but left skip so rarely volunteered that filler and bare
// acknowledgements piled into the knowledge base (1 of 6 skipped, measured
// against qwen3.5). Requiring skip as well restored it without reintroducing
// the loss. Both directions are pinned here because moving either one alone is
// what broke the other.
var skipBalanceKeep = []string{
	"다음 주 스프린트 목표는 결제 모듈 완성이야. API 설계는 내가 담당하고 프론트엔드 연동은 김대리한테 부탁하기로 했어.",
	"Go에서 goroutine leak 방지하려면 context로 cancel 전파해야 해. defer cancel() 꼭 넣어야 되고.",
	"태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 할 것 같아.",
	"오늘 점심 뭐 먹지? 김치찌개 먹을까 아니면 그냥 편의점 갈까",
	"제육볶음 만들 때 돼지고기 앞다리살 써야 맛있어. 고추장이랑 간장 비율이 2대1이야.",
	"오늘 발표 완전 망했다. 준비를 너무 못했나봐. 다음엔 더 잘 할 수 있겠지",
	// The A/B from #12: a real transcript with the hallucinated outro attached.
	"태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 할 것 같아. 시청해주셔서 감사합니다.",
}

var skipBalanceDrop = []string{
	"아 진짜요? 네 네 그렇군요 아 네.",
	"자 그러면 이제 저기 그 뭐지 아 잠시만요.",
	"네 알겠습니다.",
	"여보세요? 여보세요? 아 들리세요? 네 네.",
	"하나 둘 셋 넷 다섯 여섯 일곱 여덟.",
	"어 잠깐만요. 아 네. 어 그러니까.",
}

// TestClassifier_SkipBalance_Ollama exercises the ollama path specifically: the
// schema that decides this only applies there, and the other integration test
// runs against the Claude CLI.
func TestClassifier_SkipBalance_Ollama(t *testing.T) {
	o := NewOllamaClassifier("", "qwen3.5")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := o.Ping(ctx); err != nil {
		t.Skipf("ollama unavailable: %v", err)
	}

	categories := []string{"dev", "work", "daily"}

	for _, text := range skipBalanceKeep {
		// The pipeline strips hallucinated boilerplate before classifying, so
		// the classifier sees what it would see in production.
		cleaned := FilterHallucinations(text, nil)
		r, err := o.Classify(ctx, cleaned, categories)
		if err != nil {
			t.Fatalf("classify %q: %v", cleaned, err)
		}
		final := Finalize(r, cleaned)
		switch {
		case r.Skip:
			t.Errorf("real speech was skipped: %q", text)
		case final.Title == "" || final.Category == "":
			t.Errorf("result is not storable for %q: title=%q category=%q", text, final.Title, final.Category)
		}
	}

	for _, text := range skipBalanceDrop {
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
