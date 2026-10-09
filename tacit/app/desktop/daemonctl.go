package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sangmin7648/tacit/core/workflows/configure"
)

// cliPath is the tacit CLI bundled at Tacit.app/Contents/Helpers/tacit — not
// beside the app in MacOS, where on the default case-insensitive filesystem it
// would be the same file as Tacit.
//
// The app runs the pipeline through it rather than in-process: a daemon the
// app starts is then the same thing as one started from a terminal — a PID
// file and an event log — so one code path shows and stops either, and
// `tacit stop` works on both.
func cliPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	p := filepath.Join(filepath.Dir(exe), "..", "Helpers", "tacit")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("tacit CLI not found in the app bundle at %s — build with 'make app'", p)
	}
	return p, nil
}

// userBinDirs are where the tools the daemon shells out to — the claude CLI —
// usually live, and which launchd's PATH for Finder-launched apps omits.
var userBinDirs = []string{"$HOME/.local/bin", "/opt/homebrew/bin", "/usr/local/bin"}

// userPath returns path with userBinDirs appended where missing. They go last
// so anything already on PATH keeps precedence.
func userPath(path, home string) string {
	have := map[string]bool{}
	for _, d := range filepath.SplitList(path) {
		have[d] = true
	}
	out := path
	for _, d := range userBinDirs {
		d = strings.Replace(d, "$HOME", home, 1)
		if home == "" && strings.HasPrefix(d, "/.local") || have[d] {
			continue
		}
		if out != "" {
			out += string(os.PathListSeparator)
		}
		out += d
		have[d] = true
	}
	return out
}

// spawnListen starts `<cli> listen` with its output in logPath, which is
// truncated first: it holds the latest run's diagnostics, while the event log
// holds history. The child gets its own process group so it is not caught up
// in signals aimed at the app. The caller must Wait on the returned command.
func spawnListen(cli, logPath string) (*exec.Cmd, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening daemon log: %w", err)
	}
	defer logFile.Close() // the child holds its own descriptor once started

	cmd := exec.Command(cli, "listen")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s listen: %w", cli, err)
	}
	return cmd, nil
}

// restartTimeout bounds how long a restart waits for the daemon to exit. It
// finishes classifying what it has heard first, which takes a while.
const restartTimeout = 30 * time.Second

// ownerPath records which daemon the app started. A daemon outlives the app
// that started it when that app crashes or is killed; without the record the
// next run of the app took it for one started from a terminal, so Quit left
// it running with nothing left to show or stop it.
func ownerPath() string {
	return filepath.Join(configure.Dir(), "app-daemon.pid")
}

// recordOwned notes pid as a daemon the app started.
func recordOwned(pid int) error {
	return os.WriteFile(ownerPath(), []byte(strconv.Itoa(pid)), 0o644)
}

// ownedPID returns the PID recordOwned noted, or 0.
func ownedPID() int {
	data, err := os.ReadFile(ownerPath())
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// clearOwned drops the record once pid has stopped — only if it is pid's,
// so a daemon started in the meantime keeps its own.
func clearOwned(pid int) {
	if pid != 0 && ownedPID() == pid {
		os.Remove(ownerPath())
	}
}

// adopt returns the daemon to treat as the app's own: the running one, if the
// app started it — this run or an earlier one.
func adopt(running bool, pid int) int {
	if running && ownedPID() == pid {
		return pid
	}
	return 0
}
