// Package daemon provides PID file management with stale detection
// for the tacit daemon process.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// WritePID writes the current process PID to the specified file path.
// It creates the parent directory if it does not exist.
func WritePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating PID directory: %w", err)
	}
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

// Status reports whether the daemon recorded in the PID file at path is alive.
func Status(path string) (running bool, pid int) {
	pid, err := ReadPID(path)
	if err != nil || !IsRunning(pid) {
		return false, 0
	}
	return true, pid
}

// Stop sends SIGTERM to the running daemon. The daemon finishes classifying
// what it has already heard before it exits.
func Stop(path string) error {
	running, pid := Status(path)
	if !running {
		return errors.New("tacit is not running")
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// StopAndWait stops the running daemon and waits, up to timeout, for it to
// exit, so one started next does not find it still holding the PID file.
func StopAndWait(ctx context.Context, path string, timeout time.Duration) error {
	if err := Stop(path); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if running, _ := Status(path); !running {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("tacit is still stopping after %v", timeout)
		case <-time.After(100 * time.Millisecond):
		}
	}
}
