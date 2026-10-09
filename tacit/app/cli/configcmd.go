package main

import (
	"fmt"
	"os"

	"github.com/sangmin7648/tacit/core/workflows/configure"

	"github.com/sangmin7648/tacit/core/workflows/listen"
)

// cmdConfigSet sets one override: tacit config set <key> <value>.
func cmdConfigSet() {
	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "Usage: tacit config set <key> <value>\n")
		fmt.Fprintf(os.Stderr, "Quote values containing spaces, e.g. tacit config set initial_prompt \"tacit, whisper\"\n")
		os.Exit(1)
	}
	key, text := os.Args[3], os.Args[4]
	path := configure.OverridePath()

	value, err := configure.ParseValue(key, text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := configure.Set(key, value); err != nil {
		fmt.Fprintf(os.Stderr, "Not changed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Set %s in %s\n", key, path)
	noteRestart()
}

// cmdConfigUnset clears one override so the default applies again:
// tacit config unset <key>.
func cmdConfigUnset() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: tacit config unset <key>\n")
		os.Exit(1)
	}
	key := os.Args[3]

	if err := configure.Unset(key); err != nil {
		fmt.Fprintf(os.Stderr, "Not changed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Cleared %s; the default applies\n", key)
	noteRestart()
}

// noteRestart tells the user a running daemon won't see the change: it reads
// its config once, at startup.
func noteRestart() {
	pid, err := listen.ReadPID(listen.PIDPath())
	if err == nil && listen.IsRunning(pid) {
		fmt.Printf("tacit is running (PID %d); restart it for the change to take effect: tacit stop && tacit listen\n", pid)
	}
}
