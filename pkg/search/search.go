//go:build darwin

package search

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/sangmin7648/tacit/pkg/storage"
)

// SearchResult wraps a KnowledgeEntry with search-specific metadata.
type SearchResult struct {
	*storage.KnowledgeEntry
	Score      int
	MatchLines []string // matched lines (up to 2) for display
}

var (
	rgOnce sync.Once
	rgBin  string
	rgErr  error
)

// extractRg extracts the embedded rg binary to a temp file and returns its path.
// The extraction happens only once per process.
func extractRg() (string, error) {
	rgOnce.Do(func() {
		arch := runtime.GOARCH // "arm64" or "amd64"
		name := "rg-darwin-" + arch

		data, err := rgFS.ReadFile(name)
		if err != nil {
			rgErr = fmt.Errorf("reading embedded rg binary (%s): %w", name, err)
			return
		}

		tmpDir, err := os.MkdirTemp("", "tacit-rg-*")
		if err != nil {
			rgErr = fmt.Errorf("creating temp dir for rg: %w", err)
			return
		}

		path := filepath.Join(tmpDir, "rg")
		if err := os.WriteFile(path, data, 0755); err != nil {
			rgErr = fmt.Errorf("writing rg binary: %w", err)
			return
		}

		rgBin = path
	})
	return rgBin, rgErr
}

// rgMatchLine is a minimal subset of ripgrep's --json output schema.
type rgMatchLine struct {
	Type string `json:"type"`
	Data struct {
		Path  struct{ Text string `json:"text"` } `json:"path"`
		Lines struct{ Text string `json:"text"` } `json:"lines"`
	} `json:"data"`
}

// isFrontmatterLine reports whether a line is a YAML frontmatter field or delimiter.
// These lines are skipped in match display since the structured fields are already shown.
func isFrontmatterLine(s string) bool {
	s = strings.TrimSpace(s)
	return s == "---" ||
		strings.HasPrefix(s, "title:") ||
		strings.HasPrefix(s, "category:") ||
		strings.HasPrefix(s, "created_at:")
}

// Search searches the knowledge base at baseDir for pattern (case-insensitive).
// Returns results sorted by relevance score descending.
func Search(baseDir, pattern string) ([]*SearchResult, error) {
	rg, err := extractRg()
	if err != nil {
		return nil, err
	}

	// Single rg invocation over the entire knowledge base.
	cmd := exec.Command(rg,
		"--json",
		"-i",           // case insensitive
		"--glob", "*.md",
		"--",
		pattern,
		baseDir,
	)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// exit code 1 = no matches
			return nil, nil
		}
		return nil, fmt.Errorf("running rg: %w", err)
	}

	// Collect matched lines per file, preserving first-seen order.
	type fileData struct {
		matchLines []string
	}
	byFile := make(map[string]*fileData)
	var fileOrder []string

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var line rgMatchLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Type != "match" {
			continue
		}
		path := line.Data.Path.Text
		if path == "" {
			continue
		}
		if _, exists := byFile[path]; !exists {
			byFile[path] = &fileData{}
			fileOrder = append(fileOrder, path)
		}
		text := strings.TrimRight(line.Data.Lines.Text, "\n\r")
		if text != "" && !isFrontmatterLine(text) && len(byFile[path].matchLines) < 2 {
			byFile[path].matchLines = append(byFile[path].matchLines, text)
		}
	}

	// Compile pattern for field-level scoring.
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
	if err != nil {
		return nil, fmt.Errorf("compiling pattern: %w", err)
	}

	var results []*SearchResult
	for _, path := range fileOrder {
		fd := byFile[path]
		entry, err := storage.Read(path)
		if err != nil {
			continue
		}

		score := len(re.FindAllString(entry.Title, -1)) * 10
		score += len(re.FindAllString(entry.Category, -1)) * 5
		score += len(re.FindAllString(entry.Summary, -1)) * 3
		score += len(re.FindAllString(entry.Content, -1)) * 1

		results = append(results, &SearchResult{
			KnowledgeEntry: entry,
			Score:          score,
			MatchLines:     fd.matchLines,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}
