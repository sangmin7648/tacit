package storage

import "time"

// KnowledgeEntry represents a single knowledge item stored as a Markdown file
// with YAML frontmatter.
type KnowledgeEntry struct {
	Title     string    `yaml:"title"`
	Category  string    `yaml:"category"`
	CreatedAt time.Time `yaml:"created_at"`
	Summary   string    // Body first section (before ---)
	Content   string    // Body second section (after ---)
	FilePath  string    // Absolute file path (not stored in file, derived)
}
