package process

import (
	"strings"
	"unicode"
)

// UnsortedCategory is where transcripts land when the LLM could not classify
// them. They are stored under it rather than discarded so that nothing which
// was successfully transcribed disappears from `tacit list`.
const UnsortedCategory = "unsorted"

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

	var kept []string
	for _, sentence := range splitSentences(text) {
		norm := normalizeForMatch(sentence)
		if norm == "" {
			continue
		}
		drop := false
		for _, p := range phrases {
			if !strings.Contains(norm, p) {
				continue
			}
			if float64(len([]rune(p))) >= hallucinationCoverage*float64(len([]rune(norm))) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, sentence)
		}
	}
	return strings.TrimSpace(strings.Join(kept, " "))
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

// DeriveTitle builds a short title from the opening words of a transcript, for
// entries the classifier could not title itself.
func DeriveTitle(text string) string {
	t := strings.Join(strings.Fields(text), " ")
	if t == "" {
		return "Unclassified transcript"
	}
	const maxRunes = 40
	r := []rune(t)
	if len(r) <= maxRunes {
		return t
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

// FallbackResult builds a minimal classification for a transcript the LLM could
// not classify, so it can still be stored instead of dropped.
func FallbackResult(text string) *ClassifyResult {
	return &ClassifyResult{
		Title:    DeriveTitle(text),
		Category: UnsortedCategory,
	}
}

// TruncateForLog shortens a transcript for a log line, rune-safely.
func TruncateForLog(text string) string {
	t := strings.Join(strings.Fields(text), " ")
	const maxRunes = 120
	r := []rune(t)
	if len(r) <= maxRunes {
		return t
	}
	return string(r[:maxRunes]) + "…"
}

// maxTitleRunes mirrors the limit storage.Write enforces. Finalize truncates to
// it so a verbose model cannot make an entry unwritable.
const maxTitleRunes = 100

// NormalizeCategory reduces whatever the model returned to the single-level,
// traversal-free name storage.Write will accept. It returns "" when nothing
// usable is left; Finalize substitutes UnsortedCategory in that case.
func NormalizeCategory(category string) string {
	// Keep only the first segment: models return "dev/backend" despite being
	// told not to, and storage rejects a slash outright.
	if idx := strings.IndexAny(category, `/\`); idx >= 0 {
		category = category[:idx]
	}
	category = strings.ReplaceAll(category, "..", "")
	category = strings.TrimSpace(category)
	if category == "chat" {
		return "daily"
	}
	return category
}

// Finalize makes a classification storable. storage.Write rejects an empty or
// over-long title and an empty or multi-level category, and Usable() is true as
// soon as any one field is filled — so a result with, say, a category but no
// title passed the usable check and then died at the write, losing the
// transcript exactly the way issue #12 did. Every field is repaired here
// instead, keeping whatever the model did manage to produce.
func Finalize(r *ClassifyResult, text string) *ClassifyResult {
	if r == nil {
		return FallbackResult(text)
	}
	out := *r
	out.Title = strings.TrimSpace(out.Title)
	out.Summary = strings.TrimSpace(out.Summary)
	out.Category = NormalizeCategory(out.Category)

	if out.Title == "" {
		out.Title = DeriveTitle(text)
	}
	if n := []rune(out.Title); len(n) > maxTitleRunes {
		out.Title = strings.TrimSpace(string(n[:maxTitleRunes-1])) + "…"
	}
	if out.Category == "" {
		out.Category = UnsortedCategory
	}
	return &out
}
