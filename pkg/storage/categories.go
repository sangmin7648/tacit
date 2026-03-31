package storage

import (
	"os"
	"strings"
)

// ListCategories scans the knowledge base directory for existing category directories.
func ListCategories(baseDir string) []string {
	var categories []string

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "models" {
			continue
		}
		categories = append(categories, name)
	}

	return categories
}
