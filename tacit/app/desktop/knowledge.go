package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sangmin7648/tacit/core/workflows/browse"
	"github.com/sangmin7648/tacit/core/workflows/configure"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"
)

// storedEvent tells the browser window a new entry was stored, so it can
// refresh. frontend/src/knowledge.js subscribes to it by this exact name.
const storedEvent = "knowledge:stored"

// KnowledgeService is what the knowledge browser window calls. Listing and
// search are the browse workflow's — the same code behind `tacit list` and
// `tacit search` — so the window and the CLI find the same notes.
type KnowledgeService struct {
	mu     sync.Mutex
	window *application.WebviewWindow
}

// EntrySummary is an entry as the list shows it: everything but the
// transcript, which can be long and is fetched with Get when an entry is
// opened.
type EntrySummary struct {
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	Keywords  []string  `json:"keywords"`
	Summary   string    `json:"summary"`
	Path      string    `json:"path"`
	// MatchLines are the lines a search matched; empty when listing.
	MatchLines []string `json:"match_lines"`
}

func summarize(e *browse.Note, matches []string) EntrySummary {
	kw := e.Keywords
	if kw == nil {
		kw = []string{}
	}
	if matches == nil {
		matches = []string{}
	}
	return EntrySummary{
		Title: e.Title, Category: e.Category, CreatedAt: e.CreatedAt,
		Keywords: kw, Summary: e.Summary, Path: e.FilePath, MatchLines: matches,
	}
}

// since turns a range in days into the cutoff the browse workflow takes;
// zero or less means everything.
func since(days int) time.Time {
	if days <= 0 {
		return time.Time{}
	}
	return time.Now().AddDate(0, 0, -days)
}

// List returns the entries from the last days (all of them if days <= 0),
// newest first.
func (k *KnowledgeService) List(days int) ([]EntrySummary, error) {
	entries, err := browse.List(since(days))
	if err != nil {
		return nil, err
	}
	out := make([]EntrySummary, 0, len(entries))
	for _, e := range entries {
		out = append(out, summarize(e, nil))
	}
	return out, nil
}

// Search returns the entries from the last days matching pattern — a regular
// expression, as for `tacit search` — best match first. An empty pattern
// lists instead.
func (k *KnowledgeService) Search(pattern string, days int) ([]EntrySummary, error) {
	if strings.TrimSpace(pattern) == "" {
		return k.List(days)
	}
	results, err := browse.Search(pattern, since(days))
	if err != nil {
		return nil, err
	}
	out := make([]EntrySummary, 0, len(results))
	for _, r := range results {
		out = append(out, summarize(r.KnowledgeEntry, r.MatchLines))
	}
	return out, nil
}

// Get returns one entry in full.
func (k *KnowledgeService) Get(path string) (*browse.Note, error) {
	p, err := inKnowledgeBase(path)
	if err != nil {
		return nil, err
	}
	return browse.Read(p)
}

// Open opens an entry in the default app for Markdown.
func (k *KnowledgeService) Open(path string) error {
	p, err := inKnowledgeBase(path)
	if err != nil {
		return err
	}
	return exec.Command("open", p).Start()
}

// Reveal shows an entry in Finder.
func (k *KnowledgeService) Reveal(path string) error {
	p, err := inKnowledgeBase(path)
	if err != nil {
		return err
	}
	return exec.Command("open", "-R", p).Start()
}

// inKnowledgeBase returns path cleaned, if it names a Markdown file inside the
// knowledge base. Every method that takes a path from the window goes through
// it: the window should only ever reach the notes.
func inKnowledgeBase(path string) (string, error) {
	base, err := filepath.Abs(configure.Dir())
	if err != nil {
		return "", err
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is not in the knowledge base", path)
	}
	if filepath.Ext(p) != ".md" {
		return "", errors.New("not a knowledge entry")
	}
	return p, nil
}

// notifyStored tells the browser window, if it is open, to reload.
func (k *KnowledgeService) notifyStored() {
	k.mu.Lock()
	open := k.window != nil
	k.mu.Unlock()
	if open {
		application.Get().Event.Emit(storedEvent)
	}
}

// show opens the browser window, creating it the first time; closing hides it.
func (k *KnowledgeService) show() {
	application.InvokeSync(func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		if k.window == nil {
			w := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
				Name:      "browser",
				Title:     "Tacit Notes",
				Width:     960,
				Height:    640,
				MinWidth:  640,
				MinHeight: 400,
				URL:       "/?view=browser",
			})
			w.RegisterHook(wailsevents.Common.WindowClosing, func(e *application.WindowEvent) {
				e.Cancel()
				w.Hide()
			})
			k.window = w
		}
		application.Get().Show()
		k.window.Show()
		k.window.Focus()
	})
}
