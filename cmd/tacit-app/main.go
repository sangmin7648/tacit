// Command tacit-app is the macOS menu-bar app. It controls and watches the
// tacit daemon — the bundled CLI's `tacit listen` — and lists what it stores.
//
// It shares every decision with the CLI through pkg/*; it adds only the menu.
package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/events"
)

// pollInterval paces both the PID-file check and the event-log follow. The PID
// file changes when anything — this app, a terminal, a crash — starts or stops
// the daemon, and nothing announces it.
const pollInterval = 500 * time.Millisecond

type trayApp struct {
	app  *application.App
	tray *application.SystemTray

	mu sync.Mutex
	st state
}

func main() {
	app := application.New(application.Options{
		Name: "Tacit",
		Mac: application.MacOptions{
			// Menu bar only: no Dock icon, no app menu.
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})
	t := &trayApp{app: app, tray: app.SystemTray.New()}

	history, err := events.ReadFile(config.EventLogPath())
	if err != nil {
		log.Printf("reading event log: %v", err)
	}
	t.st.seedRecent(history)
	t.st.running, t.st.pid = daemonStatus(config.PIDPath())
	// Before Run there is no native tray yet: the tray records the label and
	// menu and applies them at startup, and there is no main thread loop to
	// dispatch to, so draw directly rather than through render.
	t.draw()

	// The watchers redraw through InvokeSync, which needs the running app.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.Event.OnApplicationEvent(wailsevents.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go t.watchPID(ctx)
		go func() {
			err := events.Follow(ctx, config.EventLogPath(), pollInterval, func(e events.Event) {
				t.update(func(s *state) { s.observe(e) })
			})
			if err != nil {
				log.Printf("following event log: %v", err)
			}
		}()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// update changes the state under the lock, then redraws.
func (t *trayApp) update(change func(*state)) {
	t.mu.Lock()
	change(&t.st)
	t.mu.Unlock()
	t.render()
}

// watchPID keeps running/pid in step with the PID file.
func (t *trayApp) watchPID(ctx context.Context) {
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		running, pid := daemonStatus(config.PIDPath())
		t.mu.Lock()
		changed := running != t.st.running || pid != t.st.pid
		if changed {
			t.st.running, t.st.pid = running, pid
			if !running {
				t.st.activity = ""
			}
		}
		t.mu.Unlock()
		if changed {
			t.render()
		}
	}
}

// render redraws the tray from any goroutine once the app is running; all
// AppKit work goes through InvokeSync.
func (t *trayApp) render() {
	application.InvokeSync(t.draw)
}

// draw rebuilds the tray label and menu from a snapshot of the state.
func (t *trayApp) draw() {
	t.mu.Lock()
	s := t.st
	s.recent = append([]events.Event(nil), t.st.recent...)
	t.mu.Unlock()

	menu := t.app.NewMenu()
	menu.Add(s.statusLine()).SetEnabled(false)
	if s.lastErr != "" {
		menu.Add(s.lastErr).SetEnabled(false)
	}
	if s.running {
		menu.Add("Stop Listening").OnClick(func(*application.Context) { t.stop() })
	} else {
		menu.Add("Start Listening").OnClick(func(*application.Context) { t.start() })
	}

	menu.AddSeparator()
	if len(s.recent) == 0 {
		menu.Add("No entries yet").SetEnabled(false)
	} else {
		menu.Add("Recent").SetEnabled(false)
		for _, e := range s.recent {
			path := e.Path
			menu.Add(entryLabel(e)).OnClick(func(*application.Context) { openPath(path) })
		}
	}

	menu.AddSeparator()
	menu.Add("Open Knowledge Folder").OnClick(func(*application.Context) { openPath(config.BaseDir()) })
	menu.Add("Open Daemon Log").OnClick(func(*application.Context) { openPath(daemonLogPath()) })
	menu.AddSeparator()
	quit := "Quit Tacit"
	if s.running && s.ownPID == s.pid {
		quit = "Quit Tacit (stops listening)"
	}
	menu.Add(quit).OnClick(func(*application.Context) { t.quit() })

	t.tray.SetLabel(s.label())
	t.tray.SetMenu(menu)
}

func (t *trayApp) start() {
	cli, err := cliPath()
	if err == nil {
		var cmd *exec.Cmd
		if cmd, err = spawnListen(cli, daemonLogPath()); err == nil {
			t.update(func(s *state) { s.ownPID, s.lastErr = cmd.Process.Pid, "" })
			go t.reap(cmd)
			return
		}
	}
	t.update(func(s *state) { s.lastErr = "Couldn't start: " + err.Error() })
}

// reap waits for a daemon this app started. An exit nobody asked for — the
// model failing to load, the classifier unreachable, a permission refused —
// is surfaced in the menu, pointing at the log that says why.
func (t *trayApp) reap(cmd *exec.Cmd) {
	err := cmd.Wait()
	pid := cmd.Process.Pid
	t.update(func(s *state) {
		if s.ownPID != pid {
			return
		}
		s.ownPID = 0
		if err != nil {
			s.lastErr = fmt.Sprintf("Stopped unexpectedly (%v) — see the daemon log", err)
		}
	})
}

func (t *trayApp) stop() {
	if err := stopDaemon(config.PIDPath()); err != nil {
		t.update(func(s *state) { s.lastErr = "Couldn't stop: " + err.Error() })
	}
}

// quit stops the daemon only if this app started it; one started from a
// terminal belongs to that terminal.
func (t *trayApp) quit() {
	t.mu.Lock()
	own := t.st.running && t.st.ownPID != 0 && t.st.ownPID == t.st.pid
	t.mu.Unlock()
	if own {
		if err := stopDaemon(config.PIDPath()); err != nil {
			log.Printf("stopping daemon on quit: %v", err)
		}
	}
	t.app.Quit()
}

func daemonLogPath() string {
	return filepath.Join(config.BaseDir(), "daemon.log")
}

func openPath(p string) {
	if err := exec.Command("open", p).Start(); err != nil {
		log.Printf("open %s: %v", p, err)
	}
}
