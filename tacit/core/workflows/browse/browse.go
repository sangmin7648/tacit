// Package browse finds the notes tacit has stored: listing, searching, and
// reading them.
package browse

import (
	"time"

	notemanager "github.com/sangmin7648/tacit/core/internal/components/note-manager"
	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
)

type (
	Note         = notemanager.KnowledgeEntry
	SearchResult = notemanager.SearchResult
)

// Dir is the folder the notes live in.
func Dir() string { return settingmanager.BaseDir() }

// List returns the notes created after since, newest first.
func List(since time.Time) ([]*Note, error) { return notemanager.ListEntries(Dir(), since) }

// Search returns the notes created after since that match pattern.
func Search(pattern string, since time.Time) ([]*SearchResult, error) {
	return notemanager.Search(Dir(), pattern, since)
}

// Read returns the note at path.
func Read(path string) (*Note, error) { return notemanager.Read(path) }
