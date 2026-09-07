package process

import (
	"strings"
	"testing"
)

// The A/B case from issue #12: real transcripts were dropped only when a
// hallucinated outro was mixed in, so the filter has to remove the outro while
// leaving the surrounding speech untouched.
func TestFilterHallucinations_RemovesOutroKeepsSpeech(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "korean outro appended to real speech",
			in:   "태그 롤백 논의를 했어. CDC 파이프라인부터 다시 봐야 할 것 같아. 시청해주셔서 감사합니다.",
			want: "태그 롤백 논의를 했어. CDC 파이프라인부터 다시 봐야 할 것 같아.",
		},
		{
			name: "outro variant with extra space",
			in:   "기획전 티어 정책 정리함. 시청해 주셔서 감사합니다!",
			want: "기획전 티어 정책 정리함.",
		},
		{
			name: "subscribe boilerplate in the middle",
			in:   "오늘 배포 나갔어. 구독과 좋아요 부탁드립니다. 롤백 계획도 세워놨고.",
			want: "오늘 배포 나갔어. 롤백 계획도 세워놨고.",
		},
		{
			name: "english outro",
			in:   "We agreed to ship the migration on Friday. Thanks for watching!",
			want: "We agreed to ship the migration on Friday.",
		},
		{
			name: "transcript with no hallucination is untouched",
			in:   "Go에서 goroutine leak 방지하려면 context로 cancel 전파해야 해.",
			want: "Go에서 goroutine leak 방지하려면 context로 cancel 전파해야 해.",
		},
		{
			name: "pure hallucination leaves nothing",
			in:   "시청해주셔서 감사합니다. 다음 영상에서 만나요.",
			want: "",
		},
		{
			name: "a sentence looped three or more times is dropped whole",
			in:   "됐어. 됐어. 됐어. 됐어. 됐어. 됐어. 됐어.",
			want: "",
		},
		{
			name: "a looped sentence is dropped but the real speech around it stays",
			in:   "태그 롤백 논의를 했어. 됐어. 됐어. 됐어. 됐어.",
			want: "태그 롤백 논의를 했어.",
		},
		{
			name: "the same sentence said just twice is left alone",
			in:   "다시 확인해봐. 다시 확인해봐.",
			want: "다시 확인해봐. 다시 확인해봐.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterHallucinations(tt.in, nil); got != tt.want {
				t.Errorf("FilterHallucinations()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// A denylisted phrase inside a longer, genuine sentence must not take the whole
// sentence with it — a false drop destroys real speech silently.
func TestFilterHallucinations_KeepsRealSentenceContainingPhrase(t *testing.T) {
	in := "발표 끝나고 팀원들한테 시청해주셔서 감사합니다 라고 말해야 하나 고민했는데 그냥 넘어갔어."
	if got := FilterHallucinations(in, nil); got != in {
		t.Errorf("real sentence was dropped\n got: %q\nwant: %q", got, in)
	}
}

func TestFilterHallucinations_ExtraPhrases(t *testing.T) {
	in := "회의 정리 끝. 오늘도 시청해주셔서 고맙습니다."
	want := "회의 정리 끝."
	if got := FilterHallucinations(in, []string{"오늘도 시청해주셔서 고맙습니다"}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizedRuneCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"감사합니다.", 5},
		{"  네, 그렇죠!  ", 4},
		{"...", 0},
		{"", 0},
		{"MBC 뉴스", 5},
	}
	for _, tt := range tests {
		if got := NormalizedRuneCount(tt.in); got != tt.want {
			t.Errorf("NormalizedRuneCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestIsFiller(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"음... 어... 그... 음... 어...", true},
		{"uh... um... uh", true},
		{"", true},
		{"...", true},
		{"음 오늘 회의 정리했어", false},
		{"Go 컨텍스트 전파", false},
	}
	for _, tt := range tests {
		if got := IsFiller(tt.in); got != tt.want {
			t.Errorf("IsFiller(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

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
			t.Errorf("empty title for input %q", TruncateForLog(text))
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
