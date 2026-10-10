package settingmanager

import (
	"slices"
	"testing"
)

func TestParseAppleLanguages(t *testing.T) {
	out := "(\n    \"en-KR\",\n    \"ko-KR\"\n)\n"
	if got, want := parseAppleLanguages(out), []string{"en-KR", "ko-KR"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := parseAppleLanguages(""); len(got) != 0 {
		t.Errorf("empty output gave %v", got)
	}
}
