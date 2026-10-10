package settingmanager

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// OnboardedPath is where the onboarding revision the user last went through is
// kept (~/.tacit/onboarded). It is not a setting: config.yaml is regenerated
// by setup and edited by the user, and a revision number in either would be
// overwritten or mistaken for something to tune.
func OnboardedPath() string {
	return filepath.Join(BaseDir(), "onboarded")
}

// OnboardedRevision returns the revision recorded by MarkOnboarded, or 0 when
// none was — a first run, or a user from before revisions existed.
func OnboardedRevision() int {
	data, err := os.ReadFile(OnboardedPath())
	if err != nil {
		return 0
	}
	rev, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || rev < 0 {
		return 0
	}
	return rev
}

// MarkOnboarded records that the user has been through onboarding revision rev.
func MarkOnboarded(rev int) error {
	if err := os.MkdirAll(BaseDir(), 0o755); err != nil {
		return err
	}
	return writeAtomic(OnboardedPath(), []byte(strconv.Itoa(rev)+"\n"))
}
