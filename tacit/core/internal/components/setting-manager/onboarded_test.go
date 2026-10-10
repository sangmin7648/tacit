package settingmanager

import (
	"os"
	"testing"
)

func TestOnboardedRevision(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := OnboardedRevision(); got != 0 {
		t.Errorf("before any marking = %d, want 0", got)
	}
	if err := MarkOnboarded(3); err != nil {
		t.Fatal(err)
	}
	if got := OnboardedRevision(); got != 3 {
		t.Errorf("after MarkOnboarded(3) = %d, want 3", got)
	}

	if err := os.WriteFile(OnboardedPath(), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := OnboardedRevision(); got != 0 {
		t.Errorf("unreadable marker = %d, want 0 so the user is asked again", got)
	}
}
