package process

import (
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/config"
)

// The corpus below is pinned from the #12 and #14 incidents: transcripts that
// actually reached the pipeline, labelled by what should have happened to
// them. Skip has failed in both directions. Before #12 the classifier
// discarded real speech; the first fix made the content fields required, which
// stopped the loss but left skip so rarely volunteered that filler and bare
// acknowledgements piled into the knowledge base (1 of 6 skipped, measured
// against qwen3.5). Requiring skip as well restored it without reintroducing
// the loss. Both directions stay pinned because moving either one alone is
// what broke the other.
//
// Two tiers read this corpus:
//
//   - the model-free contract in this file, which runs in `make test` on every
//     change and costs nothing;
//   - TestClassifier_SkipBalance_Ollama (-tags integration), which runs the
//     same cases against a live model.
//
// When an incident produces a new case, add it here and both tiers pick it up.

type corpusCase struct {
	text string
	// wantFiltered is what FilterHallucinations must leave behind. Empty means
	// the transcript has nothing to strip and must survive untouched.
	wantFiltered string
}

// corpusKeep is real speech: the deterministic gates must not eat it, and the
// classifier must not skip it.
var corpusKeep = []corpusCase{
	{text: "다음 주 스프린트 목표는 결제 모듈 완성이야. API 설계는 내가 담당하고 프론트엔드 연동은 김대리한테 부탁하기로 했어."},
	{text: "Go에서 goroutine leak 방지하려면 context로 cancel 전파해야 해. defer cancel() 꼭 넣어야 되고."},
	{text: "태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 할 것 같아."},
	{text: "오늘 점심 뭐 먹지? 김치찌개 먹을까 아니면 그냥 편의점 갈까"},
	{text: "제육볶음 만들 때 돼지고기 앞다리살 써야 맛있어. 고추장이랑 간장 비율이 2대1이야."},
	{text: "오늘 발표 완전 망했다. 준비를 너무 못했나봐. 다음엔 더 잘 할 수 있겠지"},
	// The A/B from #12: a real transcript with the hallucinated outro attached.
	// The outro goes, the speech stays.
	{
		text:         "태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 할 것 같아. 시청해주셔서 감사합니다.",
		wantFiltered: "태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 할 것 같아.",
	},
}

// corpusDrop is worthless transcript: none of it is worth a knowledge entry.
// The free gates catch none of these — every case carries real words, so
// IsFiller is false for all of them and the classifier is what decides. If the
// classifier keeps them anyway, recurrence is the gate that stops them piling
// up, which is what TestCorpusDrop_RecurrenceIsCaught pins.
var corpusDrop = []string{
	"아 진짜요? 네 네 그렇군요 아 네.",
	"자 그러면 이제 저기 그 뭐지 아 잠시만요.",
	"네 알겠습니다.",
	"여보세요? 여보세요? 아 들리세요? 네 네.",
	"하나 둘 셋 넷 다섯 여섯 일곱 여덟.",
	"어 잠깐만요. 아 네. 어 그러니까.",
}

// Real speech must reach the classifier intact. This is the #12 direction with
// no model involved: the deterministic gates get the first say, and a drop
// here destroys speech whisper already transcribed correctly.
func TestCorpusKeep_SurvivesDeterministicGates(t *testing.T) {
	for _, c := range corpusKeep {
		if IsFiller(c.text) {
			t.Errorf("real speech read as filler: %q", c.text)
			continue
		}

		want := c.wantFiltered
		if want == "" {
			want = c.text
		}
		got := FilterHallucinations(c.text, nil)
		if got != want {
			t.Errorf("FilterHallucinations(%q) = %q, want %q", c.text, got, want)
			continue
		}
		if IsFiller(got) {
			t.Errorf("filtering left only filler behind: %q -> %q", c.text, got)
		}
	}
}

// The speech-density gate must not eat real speech at its most hostile
// setting: the default rate applied to the longest segment the pipeline will
// ever hand it.
func TestCorpusKeep_NotTooSparseAtMaxSegment(t *testing.T) {
	cfg := config.DefaultConfig()
	secs := cfg.MaxSegmentDur.Seconds()

	for _, c := range corpusKeep {
		filtered := FilterHallucinations(c.text, nil)
		if TooSparse(filtered, secs, cfg.MinCharRate) {
			t.Errorf("real speech read as too sparse over %.0fs at %.2f/s (%d chars): %q",
				secs, cfg.MinCharRate, NormalizedRuneCount(filtered), c.text)
		}
	}
}

// #14: a stock sentence the classifier keeps anyway is caught by verbatim
// recurrence instead. The first dedupKeepFirst copies per window pass; the
// rest are flagged before a classify call is spent on them.
func TestCorpusDrop_RecurrenceIsCaught(t *testing.T) {
	const keepFirst = 2 // pipeline.dedupKeepFirst
	cfg := config.DefaultConfig()
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	for _, text := range corpusDrop {
		d := NewTranscriptDeduper(cfg.DedupWindow, keepFirst)

		for i := 0; i < keepFirst; i++ {
			if _, repeat := d.Seen(text, base.Add(time.Duration(i)*time.Minute)); repeat {
				t.Errorf("occurrence %d flagged as a repeat, want kept: %q", i+1, text)
			}
		}
		if _, repeat := d.Seen(text, base.Add(keepFirst*time.Minute)); !repeat {
			t.Errorf("occurrence %d not flagged as a repeat: %q", keepFirst+1, text)
		}
	}
}
