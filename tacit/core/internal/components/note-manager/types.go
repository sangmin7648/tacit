package notemanager

import "time"

// KnowledgeEntry represents a single knowledge item stored as a Markdown file
// with YAML frontmatter.
//
// The json tags are the shape `tacit list|search|get --json` print, and the
// shape a front end binds to. They are a public contract: rename one only
// alongside a bump of the CLI's JSON version.
type KnowledgeEntry struct {
	Title     string    `yaml:"title" json:"title"`
	Category  string    `yaml:"category" json:"category"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	Keywords  []string  `yaml:"keywords,omitempty" json:"keywords"`
	Summary   string    `json:"summary"` // Body first section (before ---)
	Content   string    `json:"content"` // Body second section (after ---)
	FilePath  string    `json:"path"`    // Absolute file path (not stored in file, derived)
}
