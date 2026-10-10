package settingmanager

import "regexp"

var quoted = regexp.MustCompile(`"([^"]+)"`)

// parseAppleLanguages reads the output of `defaults read -g AppleLanguages`, a
// property-list array of quoted language tags, most preferred first.
func parseAppleLanguages(out string) []string {
	var tags []string
	for _, m := range quoted.FindAllStringSubmatch(out, -1) {
		tags = append(tags, m[1])
	}
	return tags
}
