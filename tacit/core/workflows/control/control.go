// Package control finds the running daemon, stops it, and follows what it is
// doing. It never runs the daemon itself, so a front end that uses it links
// none of the audio stack; starting one is `tacit listen`.
package control

import (
	"context"
	"time"

	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
	statusreporter "github.com/sangmin7648/tacit/core/internal/components/status-reporter"
)

type (
	Event = statusreporter.Event
	Kind  = statusreporter.Kind
)

const (
	KindListening     = statusreporter.KindListening
	KindSpeechStarted = statusreporter.KindSpeechStarted
	KindSpeechEnded   = statusreporter.KindSpeechEnded
	KindTranscribing  = statusreporter.KindTranscribing
	KindTranscribed   = statusreporter.KindTranscribed
	KindDiscarded     = statusreporter.KindDiscarded
	KindClassifying   = statusreporter.KindClassifying
	KindStored        = statusreporter.KindStored
	KindSkipped       = statusreporter.KindSkipped
)

// PIDPath is the file that records the running daemon's PID.
func PIDPath() string { return settingmanager.PIDPath() }

// EventLogPath is the file the daemon reports its activity to.
func EventLogPath() string { return settingmanager.EventLogPath() }

// Status reports whether the daemon recorded at pidPath is alive.
func Status(pidPath string) (running bool, pid int) { return statusreporter.Status(pidPath) }

// ReadPID returns the PID recorded at pidPath, alive or not.
func ReadPID(pidPath string) (int, error) { return statusreporter.ReadPID(pidPath) }

// IsRunning reports whether process pid is alive.
func IsRunning(pid int) bool { return statusreporter.IsRunning(pid) }

// RemovePID deletes a PID file left behind by a daemon that is gone.
func RemovePID(pidPath string) error { return statusreporter.RemovePID(pidPath) }

// Stop asks the running daemon to exit. It finishes classifying what it has
// already heard first.
func Stop(pidPath string) error { return statusreporter.Stop(pidPath) }

// StopAndWait stops the running daemon and waits, up to timeout, for it to exit.
func StopAndWait(ctx context.Context, pidPath string, timeout time.Duration) error {
	return statusreporter.StopAndWait(ctx, pidPath, timeout)
}

// Follow calls fn for each event the daemon reports from now on, until ctx is
// cancelled.
func Follow(ctx context.Context, path string, interval time.Duration, fn func(Event)) error {
	return statusreporter.Follow(ctx, path, interval, fn)
}
