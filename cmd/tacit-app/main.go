// Command tacit-app is the macOS menu-bar app. It controls and watches the
// tacit daemon — the bundled CLI's `tacit listen` — and lists what it stores.
//
// It shares every decision with the CLI through pkg/*; it adds only the menu.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"

	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/events"
)

// frontend holds the built windows — onboarding, the notes browser and
// settings — (frontend/dist, from `npm run build`). "all:" admits the committed dist/.gitkeep, so the package compiles
// — and `make test` runs — without a frontend build; `make app` builds it.
//
//go:embed all:frontend/dist
var frontend embed.FS

// version is set at build time by the Makefile (-X main.version).
var version = "dev"

// pollInterval paces both the PID-file check and the event-log follow. The PID
// file changes when anything — this app, a terminal, a crash — starts or stops
// the daemon, and nothing announces it.
const pollInterval = 500 * time.Millisecond

type trayApp struct {
	app        *application.App
	tray       *application.SystemTray
	onboarding *OnboardingService
	knowledge  *KnowledgeService
	settings   *SettingsService

	mu sync.Mutex
	st state
}

func main() {
	// Launched from Finder, the app gets launchd's minimal PATH, and so would
	// the daemon it starts — which then cannot find the claude CLI.
	os.Setenv("PATH", userPath(os.Getenv("PATH"), os.Getenv("HOME")))

	assets, err := fs.Sub(frontend, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	onboarding := &OnboardingService{}
	knowledge := &KnowledgeService{}
	settings := &SettingsService{}
	app := application.New(application.Options{
		Name: "Tacit",
		Services: []application.Service{
			application.NewService(onboarding), application.NewService(knowledge), application.NewService(settings),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			// Menu bar only: no Dock icon, no app menu.
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})
	t := &trayApp{app: app, tray: app.SystemTray.New(), onboarding: onboarding, knowledge: knowledge, settings: settings}
	onboarding.tray, settings.tray = t, t

	history, err := events.ReadFile(config.EventLogPath())
	if err != nil {
		log.Printf("reading event log: %v", err)
	}
	t.st.seedRecent(history)
	t.st.running, t.st.pid = daemonStatus(config.PIDPath())
	t.st.ownPID = adopt(t.st.running, t.st.pid)
	t.st.updateFailed = lastUpdateFailed()
	// Before Run there is no native tray yet: the tray records the label and
	// menu and applies them at startup, and there is no main thread loop to
	// dispatch to, so draw directly rather than through render.
	t.draw()

	// The watchers redraw through InvokeSync, which needs the running app.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.Event.OnApplicationEvent(wailsevents.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		// Open onboarding on a first run — nothing is set up, so Start would
		// fail — and when a previous run of it was cut short, e.g. by the
		// relaunch macOS forces to apply Screen Recording.
		if !isConfigured() || pendingStep() > 0 {
			go onboarding.show()
		}
		go t.watchPID(ctx)
		go t.watchUpdates(ctx)
		// Reopened by an update that stopped the daemon: listen again.
		if slices.Contains(os.Args[1:], resumeFlag) && isConfigured() {
			if running, _ := daemonStatus(config.PIDPath()); !running {
				go t.start()
			}
		}
		go func() {
			err := events.Follow(ctx, config.EventLogPath(), pollInterval, func(e events.Event) {
				t.update(func(s *state) { s.observe(e) })
				if e.Kind == events.KindStored {
					knowledge.notifyStored(EntrySummary{
						Title: e.Title, Category: e.Category, CreatedAt: e.Time,
						Keywords: []string{}, Path: e.Path, MatchLines: []string{},
					})
				}
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
		if changed && t.st.running {
			clearOwned(t.st.pid) // the daemon that was running has stopped
		}
		if changed {
			t.st.running, t.st.pid = running, pid
			t.st.speakerDenied = false
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
	if s.running && s.speakerDenied {
		// A grant reaches the daemon only when it restarts, so the fix is
		// granting in Settings, then Stop and Start.
		menu.Add("System audio off: Screen Recording not granted…").OnClick(func(*application.Context) {
			t.onboarding.OpenPrivacySettings("ScreenCapture")
		})
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
	menu.Add("Browse Notes…").OnClick(func(*application.Context) { go t.knowledge.show() })
	menu.Add("Settings…").OnClick(func(*application.Context) { go t.settings.show() })
	menu.Add("Set Up Tacit…").OnClick(func(*application.Context) { go t.onboarding.show() })
	menu.Add("Open Knowledge Folder").OnClick(func(*application.Context) { openPath(config.BaseDir()) })
	menu.Add("Open Daemon Log").OnClick(func(*application.Context) { openPath(daemonLogPath()) })
	menu.AddSeparator()
	if s.upToDate {
		menu.Add("Tacit " + version + " — up to date").SetEnabled(false)
	} else {
		menu.Add("Tacit " + version).SetEnabled(false)
	}
	if isRelease(version) { // development builds don't update
		if s.updateFailed {
			menu.Add("Update failed — Open Update Log").OnClick(func(*application.Context) { openPath(updateLogPath()) })
		}
		if s.latest != "" {
			menu.Add("Update to " + s.latest + "…").OnClick(func(*application.Context) { go t.upgrade() })
		} else {
			menu.Add("Check for Updates…").OnClick(func(*application.Context) { go t.checkForUpdate(context.Background(), true) })
		}
	}
	quit := "Quit Tacit"
	if s.running && s.ownPID == s.pid {
		quit = "Quit Tacit (stops listening)"
	}
	menu.Add(quit).OnClick(func(*application.Context) { t.quit() })

	t.tray.SetLabel(s.label())
	t.tray.SetMenu(menu)
}

// start starts the daemon. A failure is shown in the menu and returned.
func (t *trayApp) start() error {
	cli, err := cliPath()
	if err == nil {
		var cmd *exec.Cmd
		if cmd, err = spawnListen(cli, daemonLogPath()); err == nil {
			if err := recordOwned(cmd.Process.Pid); err != nil {
				log.Printf("recording the daemon as the app's: %v", err)
			}
			t.update(func(s *state) { s.ownPID, s.lastErr = cmd.Process.Pid, "" })
			go t.reap(cmd)
			return nil
		}
	}
	t.update(func(s *state) { s.lastErr = "Couldn't start: " + err.Error() })
	return err
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

// quit stops the daemon only if the app started it — this run or an earlier
// one; one started from a terminal belongs to that terminal.
func (t *trayApp) quit() {
	t.mu.Lock()
	own := t.st.running && t.st.ownPID != 0 && t.st.ownPID == t.st.pid
	pid := t.st.pid
	t.mu.Unlock()
	if own {
		if err := stopDaemon(config.PIDPath()); err != nil {
			log.Printf("stopping daemon on quit: %v", err)
		} else {
			// The app exits before watchPID would see the daemon stop.
			clearOwned(pid)
		}
	}
	t.app.Quit()
}

// watchUpdates checks for a new release at startup and then daily.
func (t *trayApp) watchUpdates(ctx context.Context) {
	if !isRelease(version) {
		return
	}
	for {
		t.checkForUpdate(ctx, false)
		select {
		case <-ctx.Done():
			return
		case <-time.After(checkInterval):
		}
	}
}

// checkForUpdate looks for a release newer than this build. A failed check
// is shown only when the user asked for it.
func (t *trayApp) checkForUpdate(ctx context.Context, asked bool) {
	tag, err := latestRelease(ctx)
	if err != nil {
		log.Printf("checking for updates: %v", err)
		if asked {
			t.update(func(s *state) { s.lastErr = "Couldn't check for updates: " + err.Error() })
		}
		return
	}
	t.update(func(s *state) {
		if newer(tag, version) {
			s.latest, s.upToDate = tag, false
		} else {
			s.latest, s.upToDate = "", true
		}
	})
}

// upgrade installs the newest release. install.sh refuses to replace a
// running app or daemon, so this stops the app's daemon, hands install.sh to
// a detached updater, and quits; the updater reopens the app, which listens
// again if it was listening.
func (t *trayApp) upgrade() {
	t.mu.Lock()
	running, pid := t.st.running, t.st.pid
	own := running && t.st.ownPID != 0 && t.st.ownPID == pid
	t.mu.Unlock()
	fail := func(err error) { t.update(func(s *state) { s.lastErr = "Couldn't update: " + err.Error() }) }

	if running && !own {
		fail(errTerminalDaemon)
		return
	}
	if own {
		if err := stopAndWait(context.Background(), config.PIDPath(), restartTimeout); err != nil {
			fail(err)
			return
		}
		clearOwned(pid)
	}
	exe, err := os.Executable()
	if err != nil {
		fail(err)
		return
	}
	bundle := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "..")) // Tacit.app/Contents/MacOS/Tacit
	if err := spawnUpdater(os.Getpid(), bundle, own); err != nil {
		fail(err)
		return
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
