package transcriber

import (
	"strings"
	"unicode"
)

// hallucinatedPhrases are stock sentences whisper emits over silence and noise
// — mostly video outros baked into its training data. whisper's own
// suppress_nst only masks symbol tokens (parentheses, musical notes), so these
// arrive as ordinary words and have to be removed after transcription.
//
// Matching is case-insensitive and ignores punctuation and spacing, so a single
// entry covers its spaced and unspaced variants.
var hallucinatedPhrases = []string{
	// Korean video outros
	"시청해주셔서 감사합니다",
	"시청해 주셔서 감사합니다",
	"영상 시청해주셔서 감사합니다",
	"끝까지 시청해주셔서 감사합니다",
	"구독과 좋아요 부탁드립니다",
	"구독 좋아요 알림설정",
	"다음 영상에서 만나요",
	"다음 영상에서 뵙겠습니다",
	"다음 시간에 만나요",
	"한글자막 by 한효정",
	"이 영상은 유료광고를 포함하고 있습니다",

	// English video outros
	"thanks for watching",
	"thank you for watching",
	"thanks for watching this video",
	"please subscribe to my channel",
	"don't forget to subscribe",
	"like and subscribe",
	"see you in the next video",
	"see you next time",
	"subtitles by the amara.org community",
	"transcription by eso",
	"translation by eso",
}

// fillerTokens are the sounds STT produces for hesitation noise. A transcript
// made up of nothing but these carries no content worth storing.
var fillerTokens = map[string]bool{
	"음": true, "어": true, "그": true, "아": true, "응": true, "에": true, "저": true,
	"um": true, "uhm": true, "uh": true, "ah": true, "eh": true, "oh": true,
	"hm": true, "hmm": true, "mm": true, "mmm": true, "er": true, "erm": true,
}

// normalizeForMatch strips punctuation, whitespace and case so that phrase
// matching is insensitive to how whisper happened to punctuate a sentence.
func normalizeForMatch(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitSentences breaks a transcript into sentence-ish units. Whisper emits
// terminators inconsistently, so this also splits on newlines and treats the
// trailing fragment as a sentence.
func splitSentences(text string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		switch r {
		case '.', '!', '?', '\n', '。', '！', '？', '…':
			if s := strings.TrimSpace(cur.String()); s != "" {
				out = append(out, s)
			}
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

// hallucinationCoverage is the fraction of a sentence that must be accounted
// for by a denylisted phrase before the sentence is dropped. It keeps a real
// sentence that merely contains "thank you" from being thrown away, while
// still catching a hallucinated outro with a stray word attached.
const hallucinationCoverage = 0.6

// repeatedSentenceRun is how many times one sentence must repeat back to back
// before the run is read as a whisper decode loop and dropped whole ("됐어.
// 됐어. 됐어. …"). Two in a row stays — a person can say the same short thing
// twice — and verbatim recurrence across separate transcripts is the
// TranscriptDeduper's job, not this filter's.
const repeatedSentenceRun = 3

// FilterHallucinations removes stock hallucinated phrases from a transcript,
// sentence by sentence. extra is appended to the built-in denylist so users can
// add whatever their own setup keeps producing.
//
// A sentence is dropped only when a denylisted phrase accounts for most of it;
// this deliberately errs toward keeping text, since a false drop silently
// destroys real speech while a false keep only adds noise the LLM can ignore.
func FilterHallucinations(text string, extra []string) string {
	phrases := make([]string, 0, len(hallucinatedPhrases)+len(extra))
	for _, p := range hallucinatedPhrases {
		phrases = append(phrases, normalizeForMatch(p))
	}
	for _, p := range extra {
		if n := normalizeForMatch(p); n != "" {
			phrases = append(phrases, n)
		}
	}

	sentences := splitSentences(text)
	var kept []string
	for i := 0; i < len(sentences); {
		norm := normalizeForMatch(sentences[i])
		if norm == "" {
			i++
			continue
		}

		// Fold a run of the same sentence: three or more identical in a row is
		// a decode loop and the whole run goes; one or two fall through to the
		// denylist check below.
		runLen := 1
		for i+runLen < len(sentences) && normalizeForMatch(sentences[i+runLen]) == norm {
			runLen++
		}
		if runLen >= repeatedSentenceRun {
			i += runLen
			continue
		}

		for ; runLen > 0; runLen-- {
			sentence := sentences[i]
			i++
			n := normalizeForMatch(sentence)
			drop := false
			for _, p := range phrases {
				if !strings.Contains(n, p) {
					continue
				}
				if float64(len([]rune(p))) >= hallucinationCoverage*float64(len([]rune(n))) {
					drop = true
					break
				}
			}
			if !drop {
				kept = append(kept, sentence)
			}
		}
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

// NormalizedRuneCount returns the number of letter and digit runes in s — its
// length once punctuation, whitespace and case-only differences are removed.
// The pipeline uses it as a content-length measure for the speech-density
// gate, so a transcript that is mostly punctuation does not read as long.
func NormalizedRuneCount(s string) int {
	return len([]rune(normalizeForMatch(s)))
}

// TooSparse reports whether text carries too few characters for audioSeconds of
// audio to be genuine speech, rather than a stock phrase whisper left behind
// over a stretch it otherwise read as silence. minRate is content characters
// (letters and digits) per second; minRate <= 0 disables the check, as does a
// non-positive duration.
func TooSparse(text string, audioSeconds, minRate float64) bool {
	if minRate <= 0 || audioSeconds <= 0 {
		return false
	}
	return float64(NormalizedRuneCount(text))/audioSeconds < minRate
}

// IsFiller reports whether text carries no content beyond hesitation sounds.
// Filler is detected here rather than being left to the LLM so the decision is
// deterministic and costs nothing — the classifier is then only ever asked
// about text that already has something in it.
func IsFiller(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return true
	}
	for _, f := range fields {
		token := normalizeForMatch(f)
		if token == "" {
			continue // pure punctuation
		}
		if !fillerTokens[token] {
			return false
		}
	}
	return true
}
