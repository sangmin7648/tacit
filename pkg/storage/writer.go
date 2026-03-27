package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Write writes a KnowledgeEntry to a Markdown file with YAML frontmatter.
// It creates the category directory structure under baseDir and returns the
// absolute file path of the written file.
func Write(baseDir string, entry *KnowledgeEntry) (string, error) {
	if err := validateEntry(entry); err != nil {
		return "", err
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base directory: %w", err)
	}

	// Build category directory path
	catDir := filepath.Join(absBase, filepath.FromSlash(entry.Category))
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		return "", fmt.Errorf("creating category directory: %w", err)
	}

	// Generate filename from CreatedAt
	filename := entry.CreatedAt.Format("20060102-150405") + ".md"
	filePath := filepath.Join(catDir, filename)

	// Build file content
	content := buildFileContent(entry)

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolving file path: %w", err)
	}

	return absPath, nil
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

	// Category validation
	category := strings.TrimSpace(entry.Category)
	if category == "" {
		return fmt.Errorf("category must not be empty")
	}
	if strings.Contains(category, "..") {
		return fmt.Errorf("category must not contain path traversal (..)")
	}

	// Count slashes to enforce max 2 levels (max 1 slash)
	slashCount := strings.Count(category, "/")
	if slashCount > 1 {
		return fmt.Errorf("category must have at most 2 levels (max 1 slash), got %d slashes", slashCount)
	}

	return nil
}

func buildFileContent(entry *KnowledgeEntry) string {
	var b strings.Builder

	// YAML frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", entry.Title))
	b.WriteString(fmt.Sprintf("category: %q\n", entry.Category))
	b.WriteString(fmt.Sprintf("created_at: %q\n", entry.CreatedAt.Format("2006-01-02T15:04:05-07:00")))
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
