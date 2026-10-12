package noteclassifier

import "strings"

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
		Title: DeriveTitle(text),
	}
}

// maxTitleRunes mirrors the limit notemanager.Write enforces. Finalize truncates to
// it so a verbose model cannot make an entry unwritable.
const maxTitleRunes = 100

// Finalize makes a classification storable. notemanager.Write rejects an empty or
// over-long title, and Usable() is true as soon as any one field is filled — so
// a result with, say, a summary but no title passed the usable check and then died at the write, losing the
// transcript exactly the way issue #12 did. Every field is repaired here
// instead, keeping whatever the model did manage to produce.
func Finalize(r *ClassifyResult, text string) *ClassifyResult {
	if r == nil {
		return FallbackResult(text)
	}
	out := *r
	out.Title = strings.TrimSpace(out.Title)
	out.Summary = strings.TrimSpace(out.Summary)

	if out.Title == "" {
		out.Title = DeriveTitle(text)
	}
	if n := []rune(out.Title); len(n) > maxTitleRunes {
		out.Title = strings.TrimSpace(string(n[:maxTitleRunes-1])) + "…"
	}
	return &out
}
