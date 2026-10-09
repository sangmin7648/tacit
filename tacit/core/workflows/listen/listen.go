package listen

import (
	"context"
	"fmt"
	"log"
	"os"

	micrecorder "github.com/sangmin7648/tacit/core/internal/components/mic-recorder"
	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
)

// PIDPath is the file that records the running daemon's PID. Whoever started
// the daemon, terminal or app, this file is how everyone else finds it.
func PIDPath() string { return settingmanager.PIDPath() }

// EventLogPath is the file the daemon reports its activity to.
func EventLogPath() string { return settingmanager.EventLogPath() }

// Run is the daemon: it listens to the microphone and turns what it hears
// into notes until ctx is cancelled. It refuses to start while another daemon
// holds the PID file.
func Run(ctx context.Context, cfg *settingmanager.Config) error {
	pidPath := PIDPath()
	if err := CleanStalePID(pidPath); err != nil {
		return err
	}
	if err := WritePID(pidPath); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer RemovePID(pidPath)

	p, err := New(cfg)
	if err != nil {
		return fmt.Errorf("initializing pipeline: %w", err)
	}
	defer p.Close()

	if closeEvents := openEventLog(p); closeEvents != nil {
		defer closeEvents()
	}

	mic, err := micrecorder.New()
	if err != nil {
		return fmt.Errorf("initializing microphone: %w", err)
	}
	defer mic.Close()

	log.Printf("tacit daemon started (PID: %d)", os.Getpid())
	log.Printf("Knowledge base: %s", settingmanager.BaseDir())
	err = p.Run(ctx, []micrecorder.AudioSource{mic}, []string{"mic"})
	log.Printf("tacit daemon stopped")
	return err
}

// openEventLog attaches the event log to p, so a front end can follow this
// run. A failure here is reported and swallowed: the event log only feeds a
// display, and losing it must never cost the user a transcript. The returned
// closer is nil when the log could not be opened.
func openEventLog(p *Pipeline) func() {
	w, err := NewWriter(EventLogPath(), 0)
	if err != nil {
		log.Printf("Warning: event log unavailable: %v", err)
		return nil
	}
	p.SetObserver(w)
	return func() {
		if err := w.Err(); err != nil {
			log.Printf("Warning: event log write failed: %v", err)
		}
		w.Close()
	}
}
