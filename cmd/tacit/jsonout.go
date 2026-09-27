package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/search"
	"github.com/sangmin7648/tacit/pkg/storage"
)

// jsonVersion is stamped on every --json document. Bump it for any change a
// consumer could trip over — a renamed or retyped field. Adding a field is not
// such a change: consumers are expected to ignore keys they don't know.
const jsonVersion = 1

// The --json documents. Each is an object rather than a bare array so fields
// can be added later without breaking a consumer.

type listDoc struct {
	Version int                       `json:"version"`
	Since   time.Time                 `json:"since"`
	Entries []*storage.KnowledgeEntry `json:"entries"`
}

type searchDoc struct {
	Version int                    `json:"version"`
	Pattern string                 `json:"pattern"`
	Since   *time.Time             `json:"since,omitempty"`
	Results []*search.SearchResult `json:"results"`
}

type getDoc struct {
	Version int                       `json:"version"`
	Entries []*storage.KnowledgeEntry `json:"entries"`
	Errors  []getError                `json:"errors"`
}

type getError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type statusDoc struct {
	Version int  `json:"version"`
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
	// StartedAt is when the running daemon wrote its PID file, which it does
	// once, at startup.
	StartedAt *time.Time `json:"started_at,omitempty"`
}

// stripFlag removes every occurrence of flag from args and reports whether it
// was present. It runs before each command's own positional parsing, so
// --json can go anywhere on the command line without that parsing changing.
func stripFlag(args []string, flag string) ([]string, bool) {
	out := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == flag {
			found = true
			continue
		}
		out = append(out, a)
	}
	return out, found
}

// normalizeEntries makes an entry list safe to print: a nil list or nil
// keywords would marshal as null, and consumers should only ever see [].
func normalizeEntries(entries []*storage.KnowledgeEntry) []*storage.KnowledgeEntry {
	if entries == nil {
		return []*storage.KnowledgeEntry{}
	}
	for _, e := range entries {
		if e.Keywords == nil {
			e.Keywords = []string{}
		}
	}
	return entries
}

// printJSON writes v to stdout as indented JSON and exits on failure.
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

type configDoc struct {
	Version   int            `json:"version"`
	Reference string         `json:"reference"`
	Override  string         `json:"override"`
	Fields    []config.Field `json:"fields"`
}
