package noteclassifier

import "strings"

// UnsortedCategory is where transcripts land when the LLM could not classify
// them. They are stored under it rather than discarded so that nothing which
// was successfully transcribed disappears from `tacit list`.
const UnsortedCategory = "unsorted"

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
