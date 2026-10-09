package listen

import "strings"

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
