package noteclassifier

import (
	"strings"
	"testing"
)

// FallbackResult feeds storage.Write, which rejects an empty or over-long
// title, so it has to produce a usable one for any transcript.
func TestFallbackResult_IsStorable(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "회의 내용이 아주 길게 이어지는 경우 "
	}
	for _, text := range []string{"짧은 메모", long, "a"} {
		r := FallbackResult(text)
		if r.Title == "" {
			t.Errorf("empty title for input %q", text[:min(len(text), 40)])
		}
		if n := len([]rune(r.Title)); n > 100 {
			t.Errorf("title too long for storage.Write: %d runes", n)
		}
		if r.Category != UnsortedCategory {
			t.Errorf("category = %q, want %q", r.Category, UnsortedCategory)
		}
		if !r.Usable() {
			t.Errorf("fallback result must be usable")
		}
	}
}

// sanitizeResult used to flip an empty-but-not-skipped result into a skip,
// which is exactly how successfully transcribed speech went missing.
func TestSanitizeResult_DoesNotInventSkip(t *testing.T) {
	r := &ClassifyResult{Skip: false}
	sanitizeResult(r)
	if r.Skip {
		t.Error("sanitizeResult turned an empty result into a skip; the transcript would be discarded")
	}
	if r.Usable() {
		t.Error("an empty result must not report itself as usable")
	}
}

func TestSanitizeResult_StillCleansCategory(t *testing.T) {
	r := &ClassifyResult{Title: "t", Category: "dev/backend"}
	sanitizeResult(r)
	if r.Category != "dev" {
		t.Errorf("category = %q, want %q", r.Category, "dev")
	}

	r = &ClassifyResult{Title: "t", Category: "chat"}
	sanitizeResult(r)
	if r.Category != "daily" {
		t.Errorf("category = %q, want %q", r.Category, "daily")
	}
}

func TestUsable(t *testing.T) {
	tests := []struct {
		name string
		r    *ClassifyResult
		want bool
	}{
		{"nil", nil, false},
		{"empty", &ClassifyResult{}, false},
		{"title only", &ClassifyResult{Title: "t"}, true},
		{"category only", &ClassifyResult{Category: "dev"}, true},
		{"summary only", &ClassifyResult{Summary: "s"}, true},
	}
	for _, tt := range tests {
		if got := tt.r.Usable(); got != tt.want {
			t.Errorf("%s: Usable() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// Finalize is the last thing between a classification and storage.Write, so
// every field it produces has to satisfy that validation.
func TestFinalize_AlwaysStorable(t *testing.T) {
	const text = "태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 한다"
	tests := []struct {
		name         string
		in           *ClassifyResult
		wantCategory string
		wantTitle    string
	}{
		{"nil", nil, UnsortedCategory, ""},
		{"empty", &ClassifyResult{}, UnsortedCategory, ""},
		{"empty title keeps category", &ClassifyResult{Category: "daily", Summary: "s"}, "daily", ""},
		{"whitespace title", &ClassifyResult{Title: "   ", Category: "daily"}, "daily", ""},
		{"empty category", &ClassifyResult{Title: "제목"}, UnsortedCategory, "제목"},
		{"slash category", &ClassifyResult{Title: "제목", Category: "dev/backend"}, "dev", "제목"},
		{"backslash category", &ClassifyResult{Title: "제목", Category: `dev\backend`}, "dev", "제목"},
		{"traversal category", &ClassifyResult{Title: "제목", Category: "../../etc"}, UnsortedCategory, "제목"},
		{"chat alias", &ClassifyResult{Title: "제목", Category: "chat"}, "daily", "제목"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Finalize(tt.in, text)
			if got.Title == "" {
				t.Error("title is empty; storage.Write would reject it")
			}
			if n := len([]rune(got.Title)); n > 100 {
				t.Errorf("title is %d runes; storage.Write rejects over 100", n)
			}
			if got.Category != tt.wantCategory {
				t.Errorf("category = %q, want %q", got.Category, tt.wantCategory)
			}
			if tt.wantTitle != "" && got.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", got.Title, tt.wantTitle)
			}
		})
	}
}

func TestFinalize_TruncatesOverlongTitle(t *testing.T) {
	long := strings.Repeat("아", 400)
	got := Finalize(&ClassifyResult{Title: long, Category: "work"}, "본문")
	if n := len([]rune(got.Title)); n != 100 {
		t.Errorf("title = %d runes, want exactly the 100-rune limit", n)
	}
	if !strings.HasSuffix(got.Title, "…") {
		t.Errorf("truncated title should be marked as truncated: %q", got.Title)
	}
}

// Finalize must not overwrite a perfectly good classification.
func TestFinalize_LeavesGoodResultAlone(t *testing.T) {
	in := &ClassifyResult{Title: "제목", Summary: "요약", Category: "work", Keywords: []string{"a"}}
	got := Finalize(in, "본문")
	if got.Title != "제목" || got.Summary != "요약" || got.Category != "work" || len(got.Keywords) != 1 {
		t.Errorf("Finalize altered a valid result: %+v", got)
	}
	if in.Title != "제목" || in.Category != "work" {
		t.Errorf("Finalize mutated its argument: %+v", in)
	}
}
