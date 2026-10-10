package skillinstaller

import "os/exec"

// Agent is an AI agent tacit can install its skills for.
type Agent struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Command string `json:"-"`
}

// Agents lists the supported agents. The first is the default.
var Agents = []Agent{
	{Name: "claude", Label: "Claude Code", Command: "claude"},
}

// Installed reports whether the agent's CLI is on PATH. Skills can be
// installed for an agent that is not, but they would sit unused.
func (a Agent) Installed() bool {
	_, err := exec.LookPath(a.Command)
	return err == nil
}

// AgentNames returns the names of the supported agents.
func AgentNames() []string {
	names := make([]string, len(Agents))
	for i, a := range Agents {
		names[i] = a.Name
	}
	return names
}
