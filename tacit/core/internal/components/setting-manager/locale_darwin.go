package settingmanager

import "os/exec"

// PreferredLanguages returns the languages the user ordered in System Settings
// (BCP 47 tags such as "ko-KR"), most preferred first; nil when unreadable.
func PreferredLanguages() []string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return nil
	}
	return parseAppleLanguages(string(out))
}
