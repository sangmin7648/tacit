package process

import (
	"testing"
	"time"
)

// The first keepFirst verbatim occurrences in the window pass; the rest are
// flagged as stock repeats.
func TestTranscriptDeduper_KeepsFirstThenFlags(t *testing.T) {
	d := NewTranscriptDeduper(3*time.Hour, 2)
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	want := []bool{false, false, true, true, true}
	for i, w := range want {
		prior, got := d.Seen("감사합니다.", base.Add(time.Duration(i)*time.Minute))
		if got != w {
			t.Errorf("occurrence %d: repeat=%v (prior=%d), want %v", i+1, got, prior, w)
		}
	}
}

// Once every prior occurrence has aged out of the window, the counter starts
// over — the transcript is treated as fresh again.
func TestTranscriptDeduper_WindowExpiry(t *testing.T) {
	d := NewTranscriptDeduper(1*time.Hour, 2)
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	d.Seen("감사합니다.", base)
	d.Seen("감사합니다.", base.Add(10*time.Minute))
	if _, r := d.Seen("감사합니다.", base.Add(20*time.Minute)); !r {
		t.Fatal("third occurrence within the hour should be flagged")
	}
	if _, r := d.Seen("감사합니다.", base.Add(3*time.Hour)); r {
		t.Error("occurrence after the window cleared should not be flagged")
	}
}

// Punctuation, spacing and case differences must not let a repeat slip past.
func TestTranscriptDeduper_NormalisesText(t *testing.T) {
	d := NewTranscriptDeduper(time.Hour, 2)
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	d.Seen("감사합니다.", base)
	d.Seen("  감사합니다!  ", base.Add(time.Minute))
	if _, r := d.Seen("감사합니다", base.Add(2*time.Minute)); !r {
		t.Error("punctuation/spacing variants must count as the same transcript")
	}
}

// Distinct transcripts are tracked independently — a busy but varied hour is
// never mistaken for a loop.
func TestTranscriptDeduper_DistinctTextsIndependent(t *testing.T) {
	d := NewTranscriptDeduper(time.Hour, 2)
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	texts := []string{
		"태그 롤백 논의를 했다",
		"결제 모듈 API 설계 이야기",
		"제육볶음 고추장 비율은 2대1",
		"goroutine leak 은 context 로 막는다",
		"발표 준비를 더 해야겠다",
	}
	for i, txt := range texts {
		if _, r := d.Seen(txt, base.Add(time.Duration(i)*time.Minute)); r {
			t.Errorf("distinct transcript %d was flagged as a repeat", i+1)
		}
	}
}

// A disabled deduper — window 0, keepFirst 0, or a nil receiver — never flags.
func TestTranscriptDeduper_Disabled(t *testing.T) {
	cases := map[string]*TranscriptDeduper{
		"zero window":    NewTranscriptDeduper(0, 2),
		"zero keepFirst": NewTranscriptDeduper(time.Hour, 0),
		"nil":            nil,
	}
	for name, d := range cases {
		for i := 0; i < 10; i++ {
			if prior, r := d.Seen("감사합니다.", time.Now()); r || prior != 0 {
				t.Errorf("%s: Seen reported prior=%d repeat=%v, want 0/false", name, prior, r)
			}
		}
	}
}

// Empty (or punctuation-only) text carries no signature to match on.
func TestTranscriptDeduper_EmptyText(t *testing.T) {
	d := NewTranscriptDeduper(time.Hour, 1)
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if _, r := d.Seen("...", base.Add(time.Duration(i)*time.Minute)); r {
			t.Error("empty normalised text must not be flagged as a repeat")
		}
	}
}
