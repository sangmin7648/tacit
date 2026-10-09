package main

import (
	"reflect"
	"testing"

	"github.com/sangmin7648/tacit/core/workflows/browse"
)

func TestStripFlag(t *testing.T) {
	cases := []struct {
		in       []string
		wantArgs []string
		wantSeen bool
	}{
		{nil, []string{}, false},
		{[]string{"7d"}, []string{"7d"}, false},
		{[]string{"--json"}, []string{}, true},
		{[]string{"--json", "7d"}, []string{"7d"}, true},
		{[]string{"--duration", "1h", "--json", "meeting"}, []string{"--duration", "1h", "meeting"}, true},
		{[]string{"a.md", "--json", "b.md", "--json"}, []string{"a.md", "b.md"}, true},
	}
	for _, c := range cases {
		got, seen := stripFlag(c.in, "--json")
		if !reflect.DeepEqual(got, c.wantArgs) || seen != c.wantSeen {
			t.Errorf("stripFlag(%q) = %q, %v; want %q, %v", c.in, got, seen, c.wantArgs, c.wantSeen)
		}
	}
}

// Consumers should only ever see [], never null.
func TestNormalizeEntries(t *testing.T) {
	if got := normalizeEntries(nil); got == nil || len(got) != 0 {
		t.Errorf("normalizeEntries(nil) = %#v, want empty non-nil slice", got)
	}
	e := &browse.Note{Title: "t"}
	normalizeEntries([]*browse.Note{e})
	if e.Keywords == nil {
		t.Error("nil Keywords not replaced with an empty slice")
	}
}
