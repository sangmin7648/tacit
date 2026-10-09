package transcriber

import (
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
			name: "a run leading the transcript is dropped, trailing speech kept",
			in:   "네. 네. 네. 그러면 롤백부터 진행하자.",
			want: "그러면 롤백부터 진행하자.",
		},
		{
			name: "the same sentence said just twice is left alone",
			in:   "다시 확인해봐. 다시 확인해봐.",
			want: "다시 확인해봐. 다시 확인해봐.",
		},
		{
			name: "two separate pairs are both kept — neither is a run of three",
			in:   "확인해봐. 확인해봐. 롤백하자. 롤백하자.",
			want: "확인해봐. 확인해봐. 롤백하자. 롤백하자.",
		},
		{
			name: "a run and a denylisted outro in one transcript both go",
			in:   "쿠폰이에요? 쿠폰이에요? 쿠폰이에요? 시청해주셔서 감사합니다.",
			want: "",
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

func TestTooSparse(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		seconds float64
		minRate float64
		want    bool
	}{
		{"stock phrase over a long window", "감사합니다.", 30, 0.2, true},
		{"same phrase in a short window is fine", "감사합니다.", 10, 0.2, false},
		{"a real sentence clears the bar easily", "태그 롤백 논의를 했고 CDC 파이프라인부터 다시 봐야 한다", 12, 0.2, false},
		{"outro stripped from real speech still clears at 0.2", "태그 롤백 논의를 했어", 25, 0.2, false},
		{"news sign-off is a near miss at the default rate", "이 시각 세계였습니다.", 40, 0.2, false},
		{"the same sign-off is caught once the rate is tuned up", "이 시각 세계였습니다.", 40, 0.3, true},
		{"disabled by minRate 0", "감사합니다.", 300, 0, false},
		{"guarded against zero duration", "감사합니다.", 0, 0.2, false},
		{"exactly at the threshold is not sparse", "12345", 25, 0.2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TooSparse(tt.text, tt.seconds, tt.minRate); got != tt.want {
				t.Errorf("TooSparse(%q, %v, %v) = %v, want %v", tt.text, tt.seconds, tt.minRate, got, tt.want)
			}
		})
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
