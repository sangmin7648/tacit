package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/setup"
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
	if o.Configured || o.Choices != setup.Defaults() {
		t.Errorf("first run: configured=%v choices=%+v, want defaults", o.Configured, o.Choices)
	}
	if o.Model.Name == "" || o.Model.Present {
		t.Errorf("first run model = %+v, want the default model, not present", o.Model)
	}

	want := setup.Defaults()
	want.LLMProvider, want.LLMModel, want.Language, want.CaptureSpeaker = "claude", "sonnet", "ko", false
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

// Granting Screen Recording makes macOS quit and reopen the app. By then the
// choices are saved, so the app counts as configured — without a record of
// the unfinished run, the window would not reopen, and would start over if
// opened by hand.
func TestProgress_ResumesAfterRelaunch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &OnboardingService{}
	if _, err := s.Apply(setup.Defaults()); err != nil {
		t.Fatal(err)
	}

	// Steps before the save are not worth resuming: nothing is saved yet.
	for _, step := range []int{0, 1} {
		if err := s.SaveProgress(step); err != nil {
			t.Fatal(err)
		}
		if got := pendingStep(); got != 0 {
			t.Fatalf("after SaveProgress(%d), pendingStep = %d, want 0", step, got)
		}
	}

	if err := s.SaveProgress(3); err != nil {
		t.Fatal(err)
	}
	// A relaunch is a fresh process: only what is on disk carries over.
	o, err := (&OnboardingService{}).Options()
	if err != nil {
		t.Fatal(err)
	}
	if !o.Configured || o.ResumeStep != 3 {
		t.Errorf("after relaunch: configured=%v resume_step=%d, want true/3", o.Configured, o.ResumeStep)
	}

	s.Finish(false)
	if got := pendingStep(); got != 0 {
		t.Errorf("after Finish, pendingStep = %d, want 0", got)
	}
	if o, _ := s.Options(); o.ResumeStep != 0 {
		t.Errorf("after Finish, resume_step = %d, want 0", o.ResumeStep)
	}
}

func TestPendingStep_IgnoresGarbage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(progressPath()), 0o755)
	for _, content := range []string{"", "three", "1", "-4"} {
		os.WriteFile(progressPath(), []byte(content), 0o644)
		if got := pendingStep(); got != 0 {
			t.Errorf("pendingStep with %q = %d, want 0", content, got)
		}
	}
}

// stubSystemAudio replaces the ScreenCaptureKit check, which shows macOS's
// permission dialog each time it runs without access, and counts the runs.
func stubSystemAudio(t *testing.T, result bool) *int {
	t.Helper()
	calls := 0
	orig := checkSystemAudio
	checkSystemAudio = func() bool { calls++; return result }
	t.Cleanup(func() { checkSystemAudio = orig })
	return &calls
}

// Polling the dialog-showing check once a second put macOS's "would like to
// record this computer's screen and audio" dialog in an endless loop. The
// polled path must never run it.
func TestPermissions_NeverRunsPromptingCheck(t *testing.T) {
	calls := stubSystemAudio(t, true)
	s := &OnboardingService{}
	for i := 0; i < 20; i++ {
		s.Permissions()
	}
	if *calls != 0 {
		t.Errorf("Permissions ran the ScreenCaptureKit check %d times; it is polled every second and must never prompt", *calls)
	}
}

// Whatever the window does, the check that can show the dialog runs at most
// once per verifyMinInterval while access is missing.
func TestVerifyScreenRecording_RateLimited(t *testing.T) {
	calls := stubSystemAudio(t, false)
	s := &OnboardingService{}
	for i := 0; i < 50; i++ {
		if s.VerifyScreenRecording() {
			t.Fatal("reported allowed while the check says no")
		}
	}
	if *calls != 1 {
		t.Fatalf("50 rapid calls ran the prompting check %d times, want 1", *calls)
	}
	if s.Permissions().ScreenRecording && !screenRecordingPreflight() {
		t.Error("Permissions reports allowed after a failed verify")
	}

	// Past the interval, a user's "Check again" does run it again.
	s.screenLastRun = time.Now().Add(-verifyMinInterval)
	s.VerifyScreenRecording()
	if *calls != 2 {
		t.Errorf("after the interval: %d runs, want 2", *calls)
	}
}

// Once ScreenCaptureKit has said yes, that stands: the polled status shows it
// and no further check — and so no dialog — ever runs in this process.
func TestVerifyScreenRecording_SuccessSticks(t *testing.T) {
	calls := stubSystemAudio(t, true)
	s := &OnboardingService{}
	if !s.VerifyScreenRecording() {
		t.Fatal("verify failed with the check succeeding")
	}
	if !s.Permissions().ScreenRecording {
		t.Error("Permissions does not reflect the verified grant")
	}
	s.screenLastRun = time.Time{}
	for i := 0; i < 10; i++ {
		s.VerifyScreenRecording()
	}
	if *calls != 1 {
		t.Errorf("check ran %d times after succeeding, want 1", *calls)
	}
}

// The window's polling timer must not reach the prompting check either —
// that is the loop this all guards against. refreshPerms is what the timer
// calls.
func TestAppSvelte_PollingDoesNotVerify(t *testing.T) {
	src, err := os.ReadFile("frontend/src/App.svelte")
	if err != nil {
		t.Fatal(err)
	}
	body := regexp.MustCompile(`(?s)async function refreshPerms\(\) \{(.*?)\n  \}`).FindStringSubmatch(string(src))
	if body == nil {
		t.Fatal("refreshPerms not found in App.svelte")
	}
	if strings.Contains(body[1], "verify") {
		t.Errorf("refreshPerms (run by the 1s poll) calls verify:%s", body[1])
	}
	if !strings.Contains(string(src), "setInterval(refreshPerms,") {
		t.Error("the poll no longer runs refreshPerms; re-check what it calls")
	}
}
