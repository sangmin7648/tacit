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
	return &Pipeline{
		cfg:        config.DefaultConfig(),
		classifier: c,
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
