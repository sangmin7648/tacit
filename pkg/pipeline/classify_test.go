package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/process"
)

// fakeClassifier is a scripted process.Classifier. singleFn and batchFn decide
// what each successive call returns, so a test can reproduce a specific model
// misbehaviour (an empty result, a short batch, an outright error).
type fakeClassifier struct {
	mu          sync.Mutex
	singleCalls int
	batchCalls  int
	singleFn    func(call int, text string) (*process.ClassifyResult, error)
	batchFn     func(call int, texts []string) ([]*process.ClassifyResult, error)
	singleTexts []string
}

var _ process.Classifier = (*fakeClassifier)(nil)

func (f *fakeClassifier) Classify(ctx context.Context, text string, cats []string) (*process.ClassifyResult, error) {
	f.mu.Lock()
	f.singleCalls++
	call := f.singleCalls
	f.singleTexts = append(f.singleTexts, text)
	fn := f.singleFn
	f.mu.Unlock()
	if fn == nil {
		return &process.ClassifyResult{Title: "t", Summary: "s", Category: "dev"}, nil
	}
	return fn(call, text)
}

func (f *fakeClassifier) ClassifyBatch(ctx context.Context, texts []string, cats []string) ([]*process.ClassifyResult, error) {
	f.mu.Lock()
	f.batchCalls++
	call := f.batchCalls
	fn := f.batchFn
	f.mu.Unlock()
	if fn == nil {
		out := make([]*process.ClassifyResult, len(texts))
		for i := range out {
			out[i] = &process.ClassifyResult{Title: "t", Summary: "s", Category: "dev"}
		}
		return out, nil
	}
	return fn(call, texts)
}

func (f *fakeClassifier) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.singleCalls, f.batchCalls
}

// newTestPipeline builds a Pipeline wired to a temp knowledge dir. whisper is
// nil: these tests drive classification directly and never touch STT.
func newTestPipeline(t *testing.T, c process.Classifier) *Pipeline {
	t.Helper()
	cfg := config.DefaultConfig()
	return &Pipeline{
		cfg:        cfg,
		classifier: c,
		deduper:    process.NewTranscriptDeduper(cfg.DedupWindow, dedupKeepFirst),
		baseDir:    t.TempDir(),
	}
}

// runClassify feeds items through classifyLoop and returns once it has drained.
func runClassify(t *testing.T, p *Pipeline, items ...classifyItem) {
	t.Helper()
	ch := make(chan classifyItem, len(items))
	for _, it := range items {
		ch <- it
	}
	close(ch)
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.classifyLoop(context.Background(), ch)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("classifyLoop did not finish")
	}
}

// storedEntries returns the titles of every knowledge file under baseDir.
func storedEntries(t *testing.T, p *Pipeline) []string {
	t.Helper()
	var titles []string
	err := filepath.WalkDir(p.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "title:") {
				titles = append(titles, strings.TrimSpace(strings.TrimPrefix(line, "title:")))
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", p.baseDir, err)
	}
	return titles
}

func items(texts ...string) []classifyItem {
	out := make([]classifyItem, len(texts))
	// Distinct timestamps: the filename is derived from CreatedAt, so entries
	// sharing a second would overwrite each other and hide a lost item.
	base := time.Date(2026, 7, 23, 16, 34, 0, 0, time.UTC)
	for i, text := range texts {
		out[i] = classifyItem{text: text, timestamp: base.Add(time.Duration(i) * time.Second)}
	}
	return out
}

// Issue #12: the model answers skip=false but fills in nothing. That used to be
// rewritten into a skip and the transcript thrown away.
func TestClassifyLoop_EmptyResultIsStoredNotDropped(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return &process.ClassifyResult{Skip: false}, nil
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 한다")...)

	got := storedEntries(t, p)
	if len(got) != 1 {
		t.Fatalf("transcript was dropped: stored %d entries, want 1", len(got))
	}
	if _, err := os.Stat(filepath.Join(p.baseDir, process.UnsortedCategory)); err != nil {
		t.Errorf("expected entry under %q: %v", process.UnsortedCategory, err)
	}
}

// A classifier error must not delete the transcript either — it is retried once
// and then stored unclassified.
func TestClassifyLoop_ClassifyErrorRetriesThenStores(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return nil, fmt.Errorf("ollama timeout")
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("기획전 티어 정책 논의 내용")...)

	if single, _ := fake.counts(); single != 2 {
		t.Errorf("Classify called %d times, want 2 (initial + one retry)", single)
	}
	if got := storedEntries(t, p); len(got) != 1 {
		t.Fatalf("transcript was dropped on classify error: stored %d entries, want 1", len(got))
	}
}

// A transient error that clears on retry should classify normally.
func TestClassifyLoop_RetrySucceeds(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			if call == 1 {
				return nil, fmt.Errorf("transient")
			}
			return &process.ClassifyResult{Title: "복구된 항목", Summary: "s", Category: "work"}, nil
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("회의 내용")...)

	got := storedEntries(t, p)
	if len(got) != 1 {
		t.Fatalf("stored %d entries, want 1", len(got))
	}
	if !strings.Contains(got[0], "복구된 항목") {
		t.Errorf("title = %s, want the retried classification", got[0])
	}
	if _, err := os.Stat(filepath.Join(p.baseDir, "work")); err != nil {
		t.Errorf("expected entry under \"work\": %v", err)
	}
}

// A batch response shorter than the batch used to leave the trailing segments
// unvisited, discarding them without a trace.
func TestClassifyLoop_ShortBatchFallsBackPerItem(t *testing.T) {
	fake := &fakeClassifier{
		batchFn: func(call int, texts []string) ([]*process.ClassifyResult, error) {
			// Two results for five inputs.
			return []*process.ClassifyResult{
				{Title: "첫번째", Summary: "s", Category: "work"},
				{Title: "두번째", Summary: "s", Category: "work"},
			}, nil
		},
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return &process.ClassifyResult{Title: "개별 " + text, Summary: "s", Category: "dev"}, nil
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("하나", "둘", "셋", "넷", "다섯")...)

	got := storedEntries(t, p)
	if len(got) != 5 {
		t.Fatalf("stored %d entries, want 5 — the tail of the batch was dropped: %v", len(got), got)
	}
	if single, _ := fake.counts(); single != 3 {
		t.Errorf("individual Classify called %d times, want 3 for the missing tail", single)
	}
}

// A nil entry inside an otherwise well-sized batch must fall back too.
func TestClassifyLoop_NilBatchEntryFallsBack(t *testing.T) {
	fake := &fakeClassifier{
		batchFn: func(call int, texts []string) ([]*process.ClassifyResult, error) {
			return []*process.ClassifyResult{
				{Title: "있음", Summary: "s", Category: "work"},
				nil,
			}, nil
		},
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return &process.ClassifyResult{Title: "폴백", Summary: "s", Category: "dev"}, nil
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("하나", "둘")...)

	if got := storedEntries(t, p); len(got) != 2 {
		t.Fatalf("stored %d entries, want 2: %v", len(got), got)
	}
}

// A whole-batch failure falls back to classifying each item on its own.
func TestClassifyLoop_BatchErrorFallsBackPerItem(t *testing.T) {
	fake := &fakeClassifier{
		batchFn: func(call int, texts []string) ([]*process.ClassifyResult, error) {
			return nil, fmt.Errorf("batch parse failure")
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("하나", "둘", "셋")...)

	if got := storedEntries(t, p); len(got) != 3 {
		t.Fatalf("stored %d entries, want 3: %v", len(got), got)
	}
}

// Issue #14: a whisper stock hallucination lands verbatim over and over. The
// first couple are stored; every copy after that is dropped *before* a
// classify call is spent on it.
func TestClassifyLoop_StockRepeatDroppedAfterKeepFirst(t *testing.T) {
	fake := &fakeClassifier{}
	p := newTestPipeline(t, fake)

	const stock = "감사합니다."
	runClassify(t, p, items(stock, stock, stock, stock, stock, stock)...)

	if got := storedEntries(t, p); len(got) != dedupKeepFirst {
		t.Fatalf("stored %d entries, want %d (first copies kept, the rest dropped): %v", len(got), dedupKeepFirst, got)
	}
	// Only the two survivors reach the classifier, as a single batch — the four
	// drops cost nothing. (stored==2 with batch==1 means the batch held exactly
	// those two.)
	single, batch := fake.counts()
	if single != 0 || batch != 1 {
		t.Errorf("classifier calls: single=%d batch=%d, want 0/1 (one batch of the two survivors)", single, batch)
	}
}

// Dedup state is held on the Pipeline, so it has to carry across separate
// classifyLoop drains, not just within one batch.
func TestClassifyLoop_DedupPersistsAcrossDrains(t *testing.T) {
	p := newTestPipeline(t, &fakeClassifier{})
	base := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	mk := func(min int) classifyItem {
		return classifyItem{text: "감사합니다.", timestamp: base.Add(time.Duration(min) * time.Minute)}
	}

	runClassify(t, p, mk(0))
	runClassify(t, p, mk(1))
	runClassify(t, p, mk(2))

	if got := storedEntries(t, p); len(got) != dedupKeepFirst {
		t.Fatalf("stored %d entries, want %d — dedup must persist across drains: %v", len(got), dedupKeepFirst, got)
	}
}

// With no deduper wired, every copy is stored — the guard must be a clean
// no-op, not a nil panic.
func TestClassifyLoop_NoDeduperStoresEveryCopy(t *testing.T) {
	p := newTestPipeline(t, &fakeClassifier{})
	p.deduper = nil

	runClassify(t, p, items("감사합니다.", "감사합니다.", "감사합니다.", "감사합니다.")...)

	if got := storedEntries(t, p); len(got) != 4 {
		t.Fatalf("no deduper: stored %d entries, want 4: %v", len(got), got)
	}
}

// Dedup keys on the exact transcript, so a busy stretch of genuinely different
// speech is untouched.
func TestClassifyLoop_DistinctTranscriptsAllStored(t *testing.T) {
	p := newTestPipeline(t, &fakeClassifier{})

	runClassify(t, p, items(
		"태그 롤백 논의를 했다",
		"결제 모듈 API 설계 이야기",
		"제육볶음 고추장 비율은 2대1",
		"goroutine leak 은 context 로 막는다",
	)...)

	if got := storedEntries(t, p); len(got) != 4 {
		t.Fatalf("stored %d entries, want 4 — distinct transcripts must not be deduped: %v", len(got), got)
	}
}

// An explicit skip is still honoured — the fix must not turn tacit into a
// store-everything pipeline.
func TestClassifyLoop_ExplicitSkipIsHonoured(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return &process.ClassifyResult{Skip: true}, nil
		},
	}
	p := newTestPipeline(t, fake)

	runClassify(t, p, items("음 어 그")...)

	if got := storedEntries(t, p); len(got) != 0 {
		t.Errorf("stored %d entries for an explicit skip, want 0: %v", len(got), got)
	}
}

// A result that Usable() accepts can still be rejected by storage.Write. The
// real qwen3.5 returns exactly this — a category with an empty title — and it
// used to die at the write with only a "Write error" line, losing the
// transcript the same way issue #12 did.
func TestClassifyLoop_PartialResultIsRepairedNotDropped(t *testing.T) {
	tests := []struct {
		name         string
		result       process.ClassifyResult
		wantCategory string
	}{
		{"empty title", process.ClassifyResult{Summary: "요약", Category: "daily"}, "daily"},
		{"empty category", process.ClassifyResult{Title: "제목", Summary: "요약"}, process.UnsortedCategory},
		{"multi-level category", process.ClassifyResult{Title: "제목", Category: "dev/backend"}, "dev"},
		{"path traversal category", process.ClassifyResult{Title: "제목", Category: ".."}, process.UnsortedCategory},
		{"overlong title", process.ClassifyResult{Title: strings.Repeat("아주긴제목", 40), Category: "work"}, "work"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.result
			fake := &fakeClassifier{
				singleFn: func(call int, text string) (*process.ClassifyResult, error) { return &r, nil },
			}
			p := newTestPipeline(t, fake)

			runClassify(t, p, items("태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 한다")...)

			if got := storedEntries(t, p); len(got) != 1 {
				t.Fatalf("transcript was dropped at the write: stored %d entries, want 1", len(got))
			}
			if _, err := os.Stat(filepath.Join(p.baseDir, tt.wantCategory)); err != nil {
				t.Errorf("expected entry under %q: %v", tt.wantCategory, err)
			}
		})
	}
}
