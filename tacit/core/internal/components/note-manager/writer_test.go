package notemanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWrite_CreatesFileWithCorrectPath(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Test Title",
		CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, time.FixedZone("KST", 9*3600)),
		Summary:   "Summary text.",
		Content:   "Content text.",
	}

	path, err := Write(baseDir, entry)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if want := filepath.Join(baseDir, "20260328-143052.md"); path != want {
		t.Errorf("path = %s, want %s", path, want)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file should exist at %s", path)
	}
}

func TestWrite_EmptyTitle(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "",
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

func TestWrite_SameSecondKeepsBothNotes(t *testing.T) {
	baseDir := t.TempDir()
	at := time.Date(2026, 3, 28, 14, 30, 52, 0, time.UTC)

	first, err := Write(baseDir, &KnowledgeEntry{Title: "first", CreatedAt: at, Content: "one"})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	second, err := Write(baseDir, &KnowledgeEntry{Title: "second", CreatedAt: at, Content: "two"})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if first == second {
		t.Fatalf("second note overwrote the first at %s", first)
	}
	for path, title := range map[string]string{first: "first", second: "second"} {
		got, err := Read(path)
		if err != nil || got.Title != title {
			t.Errorf("Read(%s) = %+v, %v; want title %q", path, got, err, title)
		}
	}
}

func TestUpdate_RewritesInPlace(t *testing.T) {
	baseDir := t.TempDir()
	entry := &KnowledgeEntry{Title: "before", CreatedAt: time.Now(), Content: "one"}
	path, err := Write(baseDir, entry)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entry.Title = "after"
	if err := Update(path, entry); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	got, err := Read(path)
	if err != nil || got.Title != "after" {
		t.Errorf("Read after Update = %+v, %v; want title after", got, err)
	}
	files, _ := filepath.Glob(filepath.Join(baseDir, "*.md"))
	if len(files) != 1 {
		t.Errorf("Update left %d files, want 1", len(files))
	}
}

func TestWrite_KoreanTitle(t *testing.T) {
	baseDir := t.TempDir()

	entry := &KnowledgeEntry{
		Title:     "Go 에러 핸들링에서 sentinel error 패턴 사용",
		CreatedAt: time.Date(2026, 3, 28, 14, 30, 52, 0, time.FixedZone("KST", 9*3600)),
		Summary:   "Summary text here.",
		Content:   "Content text here.",
	}

	path, err := Write(baseDir, entry)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	got, err := Read(path)
	if err != nil || got.Title != entry.Title {
		t.Errorf("Read() = %+v, %v; want title %q", got, err, entry.Title)
	}
}
