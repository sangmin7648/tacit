package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// AgentSkillsDir returns the skills directory for the given AI agent.
func AgentSkillsDir(agent string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case "claude":
		return filepath.Join(home, ".claude", "skills"), nil
	default:
		return "", fmt.Errorf("unknown skill agent %q", agent)
	}
}

// Install copies the embedded skill files into the given agent's skills
// directory and returns the destination paths written.
func Install(agent string) ([]string, error) {
	skillsDir, err := AgentSkillsDir(agent)
	if err != nil {
		return nil, err
	}

	var installed []string
	err = fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(skillsDir, path)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}
		installed = append(installed, dest)
		return nil
	})
	if err != nil {
		return installed, err
	}
	return installed, nil
}
