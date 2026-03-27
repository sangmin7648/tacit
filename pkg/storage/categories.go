package storage

import (
	"os"
	"path/filepath"
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

		subPath := filepath.Join(baseDir, name)
		subEntries, err := os.ReadDir(subPath)
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if subEntry.IsDir() {
				categories = append(categories, name+"/"+subEntry.Name())
			}
		}
	}

	return categories
}
