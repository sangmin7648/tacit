package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteAndRead_RoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	loc := time.FixedZone("KST", 9*3600)

	original := &KnowledgeEntry{
		Title:     "Go 에러 핸들링에서 sentinel error 패턴 사용",
		Category:  "개발",
		CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, loc),
		Summary:   "Summary text here.",
		Content:   "Content text here.",
	}

	// Write
	path, err := Write(baseDir, original)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify path contains category
	if !strings.Contains(path, "개발") {
		t.Errorf("path should contain category, got %s", path)
	}

	// Read back
	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Compare fields
	if result.Title != original.Title {
		t.Errorf("Title: got %q, want %q", result.Title, original.Title)
	}
	if result.Category != original.Category {
		t.Errorf("Category: got %q, want %q", result.Category, original.Category)
	}
	if !result.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", result.CreatedAt, original.CreatedAt)
	}
	if result.Summary != original.Summary {
		t.Errorf("Summary: got %q, want %q", result.Summary, original.Summary)
	}
	if result.Content != original.Content {
		t.Errorf("Content: got %q, want %q", result.Content, original.Content)
	}
	if result.FilePath != path {
		t.Errorf("FilePath: got %q, want %q", result.FilePath, path)
	}
}

func TestWriteAndRead_Keywords_RoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	loc := time.FixedZone("KST", 9*3600)

	original := &KnowledgeEntry{
		Title:     "파이썬 데코레이터 활용",
		Category:  "개발",
		CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, loc),
		Keywords:  []string{"데코레이터", "decorator", "functools.wraps", "파이썬", "python"},
		Summary:   "Summary text here.",
		Content:   "Content text here.",
	}

	path, err := Write(baseDir, original)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if len(result.Keywords) != len(original.Keywords) {
		t.Fatalf("Keywords length: got %d, want %d", len(result.Keywords), len(original.Keywords))
	}
	for i, kw := range original.Keywords {
		if result.Keywords[i] != kw {
			t.Errorf("Keywords[%d]: got %q, want %q", i, result.Keywords[i], kw)
		}
	}
}

func TestWriteAndRead_NoKeywords_RoundTrip(t *testing.T) {
	baseDir := t.TempDir()

	original := &KnowledgeEntry{
		Title:     "No Keywords Entry",
		Category:  "개발",
		CreatedAt: time.Now(),
		Summary:   "Summary.",
		Content:   "Content.",
	}

	path, err := Write(baseDir, original)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if len(result.Keywords) != 0 {
		t.Errorf("Keywords should be empty, got %v", result.Keywords)
	}
}

func TestRead_ValidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")

	content := `---
title: "Test Entry"
category: "dev"
created_at: "2026-03-28T14:30:52+09:00"
---

This is the summary section.

---

This is the content section.
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	entry, err := Read(filePath)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if entry.Title != "Test Entry" {
		t.Errorf("Title: got %q, want %q", entry.Title, "Test Entry")
	}
	if entry.Category != "dev" {
		t.Errorf("Category: got %q, want %q", entry.Category, "dev")
	}
	if entry.Summary != "This is the summary section." {
		t.Errorf("Summary: got %q, want %q", entry.Summary, "This is the summary section.")
	}
	if entry.Content != "This is the content section." {
		t.Errorf("Content: got %q, want %q", entry.Content, "This is the content section.")
	}

	loc := time.FixedZone("KST", 9*3600)
	expectedTime := time.Date(2026, 3, 28, 14, 30, 52, 0, loc)
	if !entry.CreatedAt.Equal(expectedTime) {
		t.Errorf("CreatedAt: got %v, want %v", entry.CreatedAt, expectedTime)
	}

	absPath, _ := filepath.Abs(filePath)
	if entry.FilePath != absPath {
		t.Errorf("FilePath: got %q, want %q", entry.FilePath, absPath)
	}
}

func TestRead_WithKeywords(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")

	content := `---
title: "Test Entry"
category: "dev"
created_at: "2026-03-28T14:30:52+09:00"
keywords: ["키워드1", "keyword2", "abbrev"]
---

This is the summary section.

---

This is the content section.
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	entry, err := Read(filePath)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	want := []string{"키워드1", "keyword2", "abbrev"}
	if len(entry.Keywords) != len(want) {
		t.Fatalf("Keywords length: got %d, want %d", len(entry.Keywords), len(want))
	}
	for i, kw := range want {
		if entry.Keywords[i] != kw {
			t.Errorf("Keywords[%d]: got %q, want %q", i, entry.Keywords[i], kw)
		}
	}
}

func TestRead_FileNotFound(t *testing.T) {
	_, err := Read("/nonexistent/path/file.md")
	if err == nil {
		t.Fatal("Read() should return error for missing file")
	}
}

func TestRead_InvalidFrontmatter(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.md")

	content := "No frontmatter here.\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	_, err := Read(filePath)
	if err == nil {
		t.Fatal("Read() should return error for missing frontmatter")
	}
}

func TestRead_NoContentSeparator(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nosep.md")

	content := `---
title: "Test"
category: "dev"
created_at: "2026-03-28T14:30:52+09:00"
---

Just a summary, no content separator.
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	entry, err := Read(filePath)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if entry.Summary != "Just a summary, no content separator." {
		t.Errorf("Summary: got %q, want %q", entry.Summary, "Just a summary, no content separator.")
	}
	if entry.Content != "" {
		t.Errorf("Content: got %q, want %q", entry.Content, "")
	}
}
