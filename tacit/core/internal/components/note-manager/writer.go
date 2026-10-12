package notemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Write writes a KnowledgeEntry to a new Markdown file with YAML frontmatter.
// The file goes directly under baseDir, named after CreatedAt, and its absolute
// path is returned. A name already taken is never overwritten: a numeric suffix
// is added instead, so two notes starting in the same second both survive.
func Write(baseDir string, entry *KnowledgeEntry) (string, error) {
	if err := validateEntry(entry); err != nil {
		return "", err
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base directory: %w", err)
	}
	if err := os.MkdirAll(absBase, 0o755); err != nil {
		return "", fmt.Errorf("creating base directory: %w", err)
	}

	stem := entry.CreatedAt.Format("20060102-150405")
	content := []byte(buildFileContent(entry))
	for n := 1; ; n++ {
		name := stem + ".md"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.md", stem, n)
		}
		filePath := filepath.Join(absBase, name)
		f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("writing file: %w", err)
		}
		_, werr := f.Write(content)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return "", fmt.Errorf("writing file: %w", werr)
		}
		return filePath, nil
	}
}

// Update rewrites the existing entry file at filePath in place.
func Update(filePath string, entry *KnowledgeEntry) error {
	if err := validateEntry(entry); err != nil {
		return err
	}
	if err := os.WriteFile(filePath, []byte(buildFileContent(entry)), 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func validateEntry(entry *KnowledgeEntry) error {
	if entry == nil {
		return fmt.Errorf("entry must not be nil")
	}

	// Title validation
	title := strings.TrimSpace(entry.Title)
	if title == "" {
		return fmt.Errorf("title must not be empty")
	}
	if len([]rune(title)) > 100 {
		return fmt.Errorf("title must not exceed 100 characters")
	}

	return nil
}

func buildFileContent(entry *KnowledgeEntry) string {
	var b strings.Builder

	// YAML frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", entry.Title))
	b.WriteString(fmt.Sprintf("created_at: %q\n", entry.CreatedAt.Format("2006-01-02T15:04:05-07:00")))
	if len(entry.Keywords) > 0 {
		kwJSON, _ := json.Marshal(entry.Keywords)
		b.WriteString(fmt.Sprintf("keywords: %s\n", kwJSON))
	}
	b.WriteString("---\n")

	// Summary
	b.WriteString("\n")
	b.WriteString(entry.Summary)
	b.WriteString("\n")

	// Separator + Content
	b.WriteString("\n---\n")
	b.WriteString("\n")
	b.WriteString(entry.Content)
	b.WriteString("\n")

	return b.String()
}
