package storage

import (
	"io/fs"
	"path/filepath"
	"sort"
	"time"
)

// ListEntries walks baseDir for knowledge entries created after since, sorted
// newest first. Unreadable or malformed files are skipped rather than
// aborting the walk.
func ListEntries(baseDir string, since time.Time) ([]*KnowledgeEntry, error) {
	var entries []*KnowledgeEntry
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			// Skip internal directories
			if d.Name() == "models" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}

		entry, err := Read(path)
		if err != nil {
			return nil // skip malformed files
		}

		if entry.CreatedAt.After(since) {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	return entries, nil
}
