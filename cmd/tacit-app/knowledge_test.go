package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/storage"
)

// seedNotes writes entries into a fresh knowledge base under a temp HOME and
// returns their paths, in the order given.
func seedNotes(t *testing.T, notes ...storage.KnowledgeEntry) []string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	var paths []string
	for i := range notes {
		p, err := storage.Write(config.BaseDir(), &notes[i])
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

func note(title, category string, age time.Duration, content string) storage.KnowledgeEntry {
	return storage.KnowledgeEntry{
		Title: title, Category: category, CreatedAt: time.Now().Add(-age).Truncate(time.Second),
		Summary: title + " summary", Content: content,
	}
}

func TestList_RangeAndOrder(t *testing.T) {
	seedNotes(t,
		note("old", "work", 40*24*time.Hour, "x"),
		note("recent", "work", 2*24*time.Hour, "x"),
		note("today", "idea", time.Hour, "a long transcript"),
	)
	k := &KnowledgeService{}

	titles := func(days int) string {
		es, err := k.List(days)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range es {
			out = append(out, e.Title)
			if e.Keywords == nil || e.MatchLines == nil {
				t.Errorf("%q: nil list fields would reach the window as null", e.Title)
			}
		}
		return strings.Join(out, ",")
	}
	for days, want := range map[int]string{1: "today", 7: "today,recent", 30: "today,recent", 0: "today,recent,old"} {
		if got := titles(days); got != want {
			t.Errorf("List(%d) = %s, want %s (newest first)", days, got, want)
		}
	}
}

// The list carries no transcript; opening an entry fetches it.
func TestListOmitsTranscript_GetReturnsIt(t *testing.T) {
	paths := seedNotes(t, note("today", "idea", time.Hour, "a long transcript"))
	k := &KnowledgeService{}

	es, _ := k.List(1)
	if len(es) != 1 || es[0].Path != paths[0] {
		t.Fatalf("List = %+v", es)
	}
	e, err := k.Get(es[0].Path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e.Content != "a long transcript" || e.Title != "today" {
		t.Errorf("Get = %+v", e)
	}
}

func TestSearch(t *testing.T) {
	seedNotes(t,
		note("ranking meeting", "work", time.Hour, "we will try click-through reranking first"),
		note("lunch", "daily", 2*time.Hour, "kimchi stew or convenience store"),
		note("old reranking idea", "idea", 20*24*time.Hour, "reranking with bandits"),
	)
	k := &KnowledgeService{}

	got, err := k.Search("reranking", 7)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Title != "ranking meeting" {
		t.Fatalf("Search(reranking, 7d) = %+v, want only the recent match", got)
	}
	if len(got[0].MatchLines) == 0 {
		t.Error("a search hit carries no match lines to show")
	}

	if all, _ := k.Search("reranking", 0); len(all) != 2 {
		t.Errorf("Search(reranking, all) = %d hits, want 2", len(all))
	}
	if blank, _ := k.Search("  ", 7); len(blank) != 2 {
		t.Errorf("an empty search should list the range: got %d, want 2", len(blank))
	}
	if _, err := k.Search("(unclosed", 0); err == nil {
		t.Error("an invalid pattern returned no error")
	}
}

// Every path the window hands back is checked: it must be a note inside the
// knowledge base, whatever the window sends.
func TestPathsConfinedToKnowledgeBase(t *testing.T) {
	paths := seedNotes(t, note("today", "idea", time.Hour, "x"))
	k := &KnowledgeService{}
	base := config.BaseDir()

	outside := filepath.Join(t.TempDir(), "elsewhere.md")
	os.WriteFile(outside, []byte("---\ntitle: x\n---\n"), 0o644)
	for _, p := range []string{
		outside,
		filepath.Join(base, "..", "elsewhere.md"),
		filepath.Join(base, "idea", "..", "..", "etc.md"),
		filepath.Join(base, "config.yaml"),
		"/etc/hosts",
		base,
	} {
		if _, err := k.Get(p); err == nil {
			t.Errorf("Get(%q) succeeded; it is not a note in the knowledge base", p)
		}
		if err := k.Open(p); err == nil {
			t.Errorf("Open(%q) succeeded", p)
		}
		if err := k.Reveal(p); err == nil {
			t.Errorf("Reveal(%q) succeeded", p)
		}
	}
	if _, err := k.Get(paths[0]); err != nil {
		t.Errorf("Get on a real note failed: %v", err)
	}
}
