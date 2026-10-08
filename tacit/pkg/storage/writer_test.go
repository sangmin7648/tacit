package storage

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestWrite_CreatesFileWithCorrectPath(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Test Title",
		Category:  "dev",
		CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, time.FixedZone("KST", 9*3600)),
		Summary:   "Summary text.",
		Content:   "Content text.",
	}

	path, err := Write(baseDir, entry)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify path contains category
	if !strings.Contains(path, "dev") {
		t.Errorf("path should contain category directory, got %s", path)
	}

	// Verify filename format
	if !strings.HasSuffix(path, "20260328-143052.md") {
		t.Errorf("path should end with timestamp filename, got %s", path)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file should exist at %s", path)
	}
}

func TestWrite_InvalidCategory_WithSlash(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Test Title",
		Category:  "a/b",
		CreatedAt: time.Now(),
		Summary:   "Summary.",
		Content:   "Content.",
	}

	_, err := Write(baseDir, entry)
	if err == nil {
		t.Fatal("Write() should return error for category with slash")
	}
	if !strings.Contains(err.Error(), "no slash allowed") {
		t.Errorf("error should mention no slash allowed, got: %v", err)
	}
}

func TestWrite_EmptyTitle(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "",
		Category:  "dev",
		CreatedAt: time.Now(),
		Summary:   "Summary.",
		Content:   "Content.",
	}

	_, err := Write(baseDir, entry)
	if err == nil {
		t.Fatal("Write() should return error for empty title")
	}
	if !strings.Contains(err.Error(), "title must not be empty") {
		t.Errorf("error should mention empty title, got: %v", err)
	}
}

func TestWrite_LongTitle(t *testing.T) {
	baseDir := t.TempDir()

	longTitle := strings.Repeat("a", 101)
	entry := &KnowledgeEntry{
		Title:     longTitle,
		Category:  "dev",
		CreatedAt: time.Now(),
		Summary:   "Summary.",
		Content:   "Content.",
	}

	_, err := Write(baseDir, entry)
	if err == nil {
		t.Fatal("Write() should return error for title > 100 chars")
	}
	if !strings.Contains(err.Error(), "must not exceed 100 characters") {
		t.Errorf("error should mention character limit, got: %v", err)
	}
}

func TestWrite_PathTraversal(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Test",
		Category:  "../escape",
		CreatedAt: time.Now(),
		Summary:   "Summary.",
		Content:   "Content.",
	}

	_, err := Write(baseDir, entry)
	if err == nil {
		t.Fatal("Write() should return error for path traversal in category")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

func TestWrite_SingleLevelCategory(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Test",
		Category:  "dev",
		CreatedAt: time.Now(),
		Summary:   "Summary.",
		Content:   "Content.",
	}

	path, err := Write(baseDir, entry)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if !strings.Contains(path, "dev") {
		t.Errorf("path should contain category, got %s", path)
	}
}

func TestWrite_KoreanCategory(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Go 에러 핸들링에서 sentinel error 패턴 사용",
		Category:  "개발",
		CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, time.FixedZone("KST", 9*3600)),
		Summary:   "Summary text here.",
		Content:   "Content text here.",
	}

	path, err := Write(baseDir, entry)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if !strings.Contains(path, "개발") {
		t.Errorf("path should contain Korean category directory, got %s", path)
	}
}
