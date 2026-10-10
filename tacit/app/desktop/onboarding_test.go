package main

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/sangmin7648/tacit/core/workflows/onboard"
)

// The windows reach Go by method name, as plain strings in the frontend's
// backend files. A misspelt name fails only inside the running app, so check
// every one — both ways — and the event names, which are matched the same way.
func checkBindings(t *testing.T, file, fqnPrefix string, svcType reflect.Type) string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	js := string(src)
	if !strings.Contains(js, "const svc = '"+fqnPrefix+"'") {
		t.Errorf("%s does not address the service as %s", file, fqnPrefix)
	}

	calls := regexp.MustCompile(`svc \+ '(\w+)'`).FindAllStringSubmatch(js, -1)
	if len(calls) == 0 {
		t.Fatalf("found no service calls in %s; has the calling convention changed?", file)
	}
	called := map[string]bool{}
	for _, m := range calls {
		called[m[1]] = true
		if _, ok := svcType.MethodByName(m[1]); !ok {
			t.Errorf("%s calls %s%s, which does not exist", file, fqnPrefix, m[1])
		}
	}
	// And the other way: an exported method is bound to the window, so one
	// nothing calls is dead weight.
	for i := 0; i < svcType.NumMethod(); i++ {
		if name := svcType.Method(i).Name; !called[name] {
			t.Errorf("%s%s is bound to the window but %s never calls it", fqnPrefix, name, file)
		}
	}
	return js
}

func TestBackendJS_MatchesService(t *testing.T) {
	js := checkBindings(t, "frontend/src/backend.js", "main.OnboardingService.", reflect.TypeFor[*OnboardingService]())
	if !strings.Contains(js, `MODEL_PROGRESS = '`+modelProgressEvent+`'`) {
		t.Errorf("backend.js does not subscribe to %q", modelProgressEvent)
	}
}

func TestKnowledgeJS_MatchesService(t *testing.T) {
	js := checkBindings(t, "frontend/src/knowledge.js", "main.KnowledgeService.", reflect.TypeFor[*KnowledgeService]())
	if !strings.Contains(js, `STORED = '`+storedEvent+`'`) {
		t.Errorf("knowledge.js does not subscribe to %q", storedEvent)
	}
	if !strings.Contains(js, `SELECT = '`+selectEvent+`'`) {
		t.Errorf("knowledge.js does not subscribe to %q", selectEvent)
	}
}

// Before setup has run the form starts from the defaults; after, from what the
// user chose.
func TestOptions_PrefillsFromConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &OnboardingService{}

	o, err := s.Options()
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	if o.Configured || o.Choices != onboard.Defaults() {
		t.Errorf("first run: configured=%v choices=%+v, want defaults", o.Configured, o.Choices)
	}
	if o.Model.Name == "" || o.Model.Present {
		t.Errorf("first run model = %+v, want the default model, not present", o.Model)
	}

	want := onboard.Defaults()
	want.LLMProvider, want.LLMModel, want.Language = "claude", "sonnet", "ko"
	if _, err := s.Apply(want); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	o, err = s.Options()
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	if !o.Configured || o.Choices != want {
		t.Errorf("after setup: configured=%v choices=%+v, want %+v", o.Configured, o.Choices, want)
	}
}

func TestOpenPrivacySettings_OnlyKnownPanes(t *testing.T) {
	if err := (&OnboardingService{}).OpenPrivacySettings("../../etc"); err == nil {
		t.Error("accepted an unknown pane")
	}
}

func TestUserPath(t *testing.T) {
	cases := []struct {
		path, home, want string
	}{
		// launchd's PATH for a Finder-launched app.
		{"/usr/bin:/bin:/usr/sbin:/sbin", "/Users/me",
			"/usr/bin:/bin:/usr/sbin:/sbin:/Users/me/.local/bin:/opt/homebrew/bin:/usr/local/bin"},
		// A terminal launch already has them: nothing added, order kept.
		{"/opt/homebrew/bin:/Users/me/.local/bin:/usr/bin:/usr/local/bin", "/Users/me",
			"/opt/homebrew/bin:/Users/me/.local/bin:/usr/bin:/usr/local/bin"},
		{"", "/Users/me", "/Users/me/.local/bin:/opt/homebrew/bin:/usr/local/bin"},
		// No HOME: skip the per-user dir rather than add "/.local/bin".
		{"/usr/bin", "", "/usr/bin:/opt/homebrew/bin:/usr/local/bin"},
	}
	for _, c := range cases {
		if got := userPath(c.path, c.home); got != c.want {
			t.Errorf("userPath(%q, %q)\n got %q\nwant %q", c.path, c.home, got, c.want)
		}
	}
}
