package listen

import (
	"sync"
	"testing"
	"time"

	noteclassifier "github.com/sangmin7648/tacit/core/internal/components/note-classifier"
)

// recorder collects every event the pipeline emits. The pipeline emits from
// several goroutines, so it locks.
type recorder struct {
	mu  sync.Mutex
	got []Event
}

func (r *recorder) Observe(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, e)
}

func (r *recorder) kinds() []Kind {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Kind, len(r.got))
	for i, e := range r.got {
		out[i] = e.Kind
	}
	return out
}

func (r *recorder) first(k Kind) (Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.got {
		if e.Kind == k {
			return e, true
		}
	}
	return Event{}, false
}

func (r *recorder) count(k Kind) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.got {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func sourcedItems(source string, texts ...string) []classifyItem {
	out := items(texts...)
	for i := range out {
		out[i].source = source
	}
	return out
}

// A stored event is what tells a front end to refresh its notes.
func TestClassifyLoop_EmitsStored(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*noteclassifier.ClassifyResult, error) {
			return &noteclassifier.ClassifyResult{Title: "검색 랭킹 논의", Summary: "s", Category: "work"}, nil
		},
	}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	runClassify(t, p, sourcedItems("mic", "검색 랭킹 개선 논의")...)

	e, ok := rec.first(KindStored)
	if !ok {
		t.Fatalf("no %q event; got %v", KindStored, rec.kinds())
	}
	if e.Source != "mic" {
		t.Errorf("Source = %q, want %q", e.Source, "mic")
	}
	if e.Time.IsZero() {
		t.Error("Time is zero; emit must stamp every event")
	}
	if got := rec.count(KindClassifying); got != 1 {
		t.Errorf("classifying events = %d, want 1", got)
	}
}

// A skip ends the work on a transcript, so the menu bar must hear of it to
// stop showing it as in progress.
func TestClassifyLoop_EmitsSkippedNotStored(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*noteclassifier.ClassifyResult, error) {
			return &noteclassifier.ClassifyResult{Skip: true}, nil
		},
	}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	runClassify(t, p, sourcedItems("mic", "어 그래 응")...)

	e, ok := rec.first(KindSkipped)
	if !ok {
		t.Fatalf("no %q event; got %v", KindSkipped, rec.kinds())
	}
	if e.Source != "mic" {
		t.Errorf("Source = %q, want %q", e.Source, "mic")
	}
	if n := rec.count(KindStored); n != 0 {
		t.Errorf("stored events = %d, want 0", n)
	}
}

// The deduper drops a stock hallucination before a classify call is spent on
// it; the menu bar needs to hear that the transcript's work is over.
func TestClassifyLoop_EmitsDiscardedOnStockRepeat(t *testing.T) {
	fake := &fakeClassifier{}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	const stock = "시청해주셔서 감사합니다"
	// dedupKeepFirst copies pass; the next is treated as a stock repeat.
	for i := 0; i < dedupKeepFirst+1; i++ {
		runClassify(t, p, sourcedItems("mic", stock)...)
	}

	if _, ok := rec.first(KindDiscarded); !ok {
		t.Fatalf("no %q event; got %v", KindDiscarded, rec.kinds())
	}
	if got := rec.count(KindStored); got != dedupKeepFirst {
		t.Errorf("stored events = %d, want %d", got, dedupKeepFirst)
	}
}

// A classify failure still stores the transcript unclassified (issue #12), so
// the front end hears of the entry like any other.
func TestClassifyLoop_ClassifyFailureStillEmitsStored(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*noteclassifier.ClassifyResult, error) {
			return nil, errBoom
		},
	}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	runClassify(t, p, sourcedItems("mic", "기획전 티어 정책 논의")...)

	if n := rec.count(KindStored); n != 1 {
		t.Errorf("stored events = %d, want 1 (a classify failure must not lose the transcript)", n)
	}
}

// Pipelines built by the other tests in this package leave observer nil, and a
// Pipeline is a plain struct anyone can construct that way.
func TestEmit_NilObserverIsSafe(t *testing.T) {
	p := &Pipeline{}
	p.emit(Event{Kind: KindListening})
}

func TestSetObserver_NilRestoresDiscard(t *testing.T) {
	p := &Pipeline{}
	p.SetObserver(nil)
	p.emit(Event{Kind: KindListening, Time: time.Now()})
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "ollama timeout" }
