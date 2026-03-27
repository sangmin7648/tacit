package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Read reads a Markdown knowledge file and parses it into a KnowledgeEntry.
// It expects a file with YAML frontmatter (between the first pair of ---),
// followed by a summary section, a --- separator, and a content section.
func Read(filePath string) (*KnowledgeEntry, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolving file path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	raw := string(data)

	// Parse frontmatter: file must start with "---\n"
	if !strings.HasPrefix(raw, "---\n") {
		return nil, fmt.Errorf("file does not start with YAML frontmatter delimiter (---)")
	}

	// Find the closing frontmatter delimiter
	rest := raw[4:] // skip opening "---\n"
	closingIdx := strings.Index(rest, "\n---\n")
	if closingIdx < 0 {
		return nil, fmt.Errorf("missing closing YAML frontmatter delimiter (---)")
	}

	frontmatterStr := rest[:closingIdx]
	body := rest[closingIdx+4:] // skip "\n---\n"

	// Parse YAML frontmatter
	var entry KnowledgeEntry
	if err := yaml.Unmarshal([]byte(frontmatterStr), &entry); err != nil {
		return nil, fmt.Errorf("parsing YAML frontmatter: %w", err)
	}

	// Split body into summary and content by the "---" separator
	// Look for a line that is exactly "---"
	bodySepIdx := strings.Index(body, "\n---\n")
	if bodySepIdx >= 0 {
		entry.Summary = strings.TrimSpace(body[:bodySepIdx])
		entry.Content = strings.TrimSpace(body[bodySepIdx+4:])
	} else {
		// No separator found; treat entire body as summary
		entry.Summary = strings.TrimSpace(body)
		entry.Content = ""
	}

	entry.FilePath = absPath

	return &entry, nil
}
