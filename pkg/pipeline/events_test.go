package pipeline

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/events"
	"github.com/sangmin7648/tacit/pkg/process"
)

// recorder collects every event the pipeline emits. The pipeline emits from
// several goroutines, so it locks.
type recorder struct {
	mu  sync.Mutex
	got []events.Event
}

func (r *recorder) Observe(e events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, e)
}

func (r *recorder) kinds() []events.Kind {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]events.Kind, len(r.got))
	for i, e := range r.got {
		out[i] = e.Kind
	}
	return out
}

func (r *recorder) first(k events.Kind) (events.Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.got {
		if e.Kind == k {
			return e, true
		}
	}
	return events.Event{}, false
}

func (r *recorder) count(k events.Kind) int {
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

// A stored entry is the event a front end exists to show. It has to carry
// enough to render a row without reopening the file.
func TestClassifyLoop_EmitsStored(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return &process.ClassifyResult{Title: "검색 랭킹 논의", Summary: "s", Category: "work"}, nil
		},
	}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	runClassify(t, p, sourcedItems("mic", "검색 랭킹 개선 논의")...)

	e, ok := rec.first(events.KindStored)
	if !ok {
		t.Fatalf("no %q event; got %v", events.KindStored, rec.kinds())
	}
	if e.Title != "검색 랭킹 논의" {
		t.Errorf("Title = %q, want %q", e.Title, "검색 랭킹 논의")
	}
	if e.Category != "work" {
		t.Errorf("Category = %q, want %q", e.Category, "work")
	}
	if e.Source != "mic" {
		t.Errorf("Source = %q, want %q", e.Source, "mic")
	}
	if !strings.HasSuffix(e.Path, ".md") {
		t.Errorf("Path = %q, want a .md file", e.Path)
	}
	if e.Time.IsZero() {
		t.Error("Time is zero; emit must stamp every event")
	}
	if got := rec.count(events.KindClassifying); got != 1 {
		t.Errorf("classifying events = %d, want 1", got)
	}
}

// A skip is the one path that intentionally throws speech away, so it must be
// visible as its own kind rather than as a silent absence of a stored event.
func TestClassifyLoop_EmitsSkippedNotStored(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return &process.ClassifyResult{Skip: true}, nil
		},
	}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	runClassify(t, p, sourcedItems("mic", "어 그래 응")...)

	e, ok := rec.first(events.KindSkipped)
	if !ok {
		t.Fatalf("no %q event; got %v", events.KindSkipped, rec.kinds())
	}
	if e.Source != "mic" {
		t.Errorf("Source = %q, want %q", e.Source, "mic")
	}
	if e.Text == "" {
		t.Error("Text is empty; a skip event is the only record of what was discarded")
	}
	if n := rec.count(events.KindStored); n != 0 {
		t.Errorf("stored events = %d, want 0", n)
	}
}

// The deduper drops a stock hallucination before a classify call is spent on
// it. That drop is invisible in the stored entries, so it needs an event.
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

	e, ok := rec.first(events.KindDiscarded)
	if !ok {
		t.Fatalf("no %q event; got %v", events.KindDiscarded, rec.kinds())
	}
	if e.Reason != "stock_repeat" {
		t.Errorf("Reason = %q, want %q", e.Reason, "stock_repeat")
	}
	if got := rec.count(events.KindStored); got != dedupKeepFirst {
		t.Errorf("stored events = %d, want %d", got, dedupKeepFirst)
	}
}

// A classify failure still stores the transcript unclassified (issue #12), and
// the front end needs both facts: the error and the entry.
func TestClassifyLoop_EmitsErrorAndStillStores(t *testing.T) {
	fake := &fakeClassifier{
		singleFn: func(call int, text string) (*process.ClassifyResult, error) {
			return nil, errBoom
		},
	}
	p := newTestPipeline(t, fake)
	rec := &recorder{}
	p.SetObserver(rec)

	runClassify(t, p, sourcedItems("mic", "기획전 티어 정책 논의")...)

	e, ok := rec.first(events.KindError)
	if !ok {
		t.Fatalf("no %q event; got %v", events.KindError, rec.kinds())
	}
	if e.Reason != "classify_failed" {
		t.Errorf("Reason = %q, want %q", e.Reason, "classify_failed")
	}
	if n := rec.count(events.KindStored); n != 1 {
		t.Errorf("stored events = %d, want 1 (a classify failure must not lose the transcript)", n)
	}
}

// Pipelines built by the other tests in this package leave observer nil, and a
// Pipeline is a plain struct anyone can construct that way.
func TestEmit_NilObserverIsSafe(t *testing.T) {
	p := &Pipeline{}
	p.emit(events.Event{Kind: events.KindListening})
}

func TestSetObserver_NilRestoresDiscard(t *testing.T) {
	p := &Pipeline{}
	p.SetObserver(nil)
	p.emit(events.Event{Kind: events.KindListening, Time: time.Now()})
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "ollama timeout" }
