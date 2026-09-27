package storage

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"
)

// The json keys are what `tacit list|search|get --json` print and what a front
// end binds to. This pins them: a rename here is a breaking change and has to
// come with a bump of the CLI's JSON version.
func TestKnowledgeEntry_JSONKeys(t *testing.T) {
	e := &KnowledgeEntry{
		Title:     "t",
		Category:  "work",
		CreatedAt: time.Date(2026, 9, 21, 9, 16, 42, 0, time.UTC),
		Keywords:  []string{"a"},
		Summary:   "s",
		Content:   "c",
		FilePath:  "/x/work/20260921-091642.md",
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "category,content,created_at,keywords,path,summary,title"
	if got := strings.Join(keys, ","); got != want {
		t.Errorf("json keys = %s, want %s", got, want)
	}
	if m["path"] != e.FilePath {
		t.Errorf("path = %v, want %v", m["path"], e.FilePath)
	}
	if m["created_at"] != "2026-09-21T09:16:42Z" {
		t.Errorf("created_at = %v, want RFC 3339", m["created_at"])
	}
}
