// Package daemon provides PID file management with stale detection
// for the sttdb daemon process.
package daemon

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the current process PID to the specified file path.
func WritePID(path string) error {
	pid := os.Getpid()
	data := []byte(strconv.Itoa(pid) + "\n")
	return os.WriteFile(path, data, 0644)
}

// ReadPID reads and parses a PID from the specified file path.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return 0, fmt.Errorf("PID file is empty")
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID content %q: %w", pidStr, err)
	}

	if pid <= 0 {
		return 0, fmt.Errorf("invalid PID value: %d", pid)
	}

	return pid, nil
}

// IsRunning checks if a process with the given PID is alive by sending
// signal 0. Returns true if the process exists and is reachable.
func IsRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// RemovePID removes the PID file at the specified path.
func RemovePID(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing PID file: %w", err)
	}
	return nil
}

// CleanStalePID checks if a PID file exists and whether the referenced
// process is still running. If the process is not running, the stale PID
// file is removed. If the process is running, an error is returned
// indicating the daemon is already running. If no PID file exists,
// no action is taken and nil is returned.
func CleanStalePID(path string) error {
	pid, err := ReadPID(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		// PID file exists but is invalid; treat as stale.
		return RemovePID(path)
	}

	if IsRunning(pid) {
		return fmt.Errorf("daemon is already running (PID %d)", pid)
	}

	return RemovePID(path)
}
