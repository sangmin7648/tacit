package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}

	expected := os.Getpid()
	if pid != expected {
		t.Errorf("PID mismatch: got %d, want %d", pid, expected)
	}
}

func TestIsRunning_CurrentProcess(t *testing.T) {
	pid := os.Getpid()
	if !IsRunning(pid) {
		t.Errorf("expected current process (PID %d) to be running", pid)
	}
}

func TestIsRunning_NonExistent(t *testing.T) {
	if IsRunning(999999) {
		t.Error("expected PID 999999 to not be running")
	}
}

func TestRemovePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected PID file to be removed, but it still exists")
	}
}

func TestRemovePID_NonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")

	if err := RemovePID(path); err != nil {
		t.Errorf("RemovePID on nonexistent file should not error, got: %v", err)
	}
}

func TestCleanStalePID_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")

	if err := CleanStalePID(path); err != nil {
		t.Errorf("CleanStalePID with no file should not error, got: %v", err)
	}
}

func TestCleanStalePID_StaleProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	// Write a PID that is very unlikely to be running.
	if err := os.WriteFile(path, []byte("999999\n"), 0644); err != nil {
		t.Fatalf("failed to write stale PID file: %v", err)
	}

	if err := CleanStalePID(path); err != nil {
		t.Fatalf("CleanStalePID should remove stale PID, got error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected stale PID file to be removed")
	}
}

func TestCleanStalePID_RunningProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	err := CleanStalePID(path)
	if err == nil {
		t.Fatal("CleanStalePID should return error for running process")
	}

	// PID file should still exist since the process is running.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("PID file should still exist for a running process")
	}
}

func TestCleanStalePID_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0644); err != nil {
		t.Fatalf("failed to write invalid PID file: %v", err)
	}

	// Invalid content should be treated as stale and removed.
	if err := CleanStalePID(path); err != nil {
		t.Fatalf("CleanStalePID should remove invalid PID file, got error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected invalid PID file to be removed")
	}
}

func TestReadPID_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")

	_, err := ReadPID(path)
	if err == nil {
		t.Error("ReadPID should return error for missing file")
	}
}

func TestReadPID_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pid")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty PID file: %v", err)
	}

	_, err := ReadPID(path)
	if err == nil {
		t.Error("ReadPID should return error for empty file")
	}
}

func TestReadPID_NegativePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "negative.pid")

	if err := os.WriteFile(path, []byte("-1\n"), 0644); err != nil {
		t.Fatalf("failed to write negative PID file: %v", err)
	}

	_, err := ReadPID(path)
	if err == nil {
		t.Error("ReadPID should return error for negative PID")
	}
}
