// Command Tacit is the macOS menu-bar app. It controls and watches the
// tacit daemon — the bundled CLI's `tacit listen` — and lists what it stores.
//
// It shares every decision with the CLI through core/workflows; it adds only the menu.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/sangmin7648/tacit/core/workflows/control"

	"github.com/sangmin7648/tacit/core/workflows/browse"
	"github.com/sangmin7648/tacit/core/workflows/configure"
	"github.com/sangmin7648/tacit/core/workflows/onboard"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"
)

// frontend holds the built windows — onboarding, the notes browser and
// settings — (frontend/dist, from `npm run build`). "all:" admits the committed dist/.gitkeep, so the package compiles
// — and `make test` runs — without a frontend build; `make app` builds it.
//
//go:embed all:frontend/dist
var frontend embed.FS

// trayIcons holds the menu-bar images state.icon names. They are template
// images: macOS tints them for light and dark menu bars.
//
//go:embed icons/*.png
var trayIcons embed.FS

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

	t.st.recent = recentEntries()
	t.st.running, t.st.pid = control.Status(control.PIDPath())
	t.st.ownPID = adopt(t.st.running, t.st.pid)
	t.st.updateFailed = lastUpdateFailed()
	// Before Run there is no native tray yet: the tray records the icon and
	// menu and applies them at startup, and there is no main thread loop to
	// dispatch to, so draw directly rather than through render.
	t.draw()

	// The watchers redraw through InvokeSync, which needs the running app.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.Event.OnApplicationEvent(wailsevents.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		// Open onboarding on a first run (nothing is set up, so Start would
		// fail) and once per revision for users already set up. Those are marked
		// at once: closing the window must not bring it back at every launch.
		if onboard.Needed() {
			if isConfigured() {
				onboard.MarkSeen()
			}
			go onboarding.show()
		}
		go t.watchPID(ctx)
		go t.watchUpdates(ctx)
		go t.animate(ctx)
		// Reopened by an update that stopped the daemon: listen again.
		if slices.Contains(os.Args[1:], resumeFlag) && isConfigured() {
			if running, _ := control.Status(control.PIDPath()); !running {
				go t.start()
			}
		}
		go func() {
			err := control.Follow(ctx, control.EventLogPath(), pollInterval, func(e control.Event) {
				if e.Kind != control.KindStored {
					t.update(func(s *state) { s.observe(e) })
					return
				}
				recent := recentEntries()
				t.update(func(s *state) { s.observe(e); s.recent = recent })
				knowledge.notifyStored()
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
		running, pid := control.Status(control.PIDPath())
		t.mu.Lock()
		changed := running != t.st.running || pid != t.st.pid
		if changed && t.st.running {
			clearOwned(t.st.pid) // the daemon that was running has stopped
		}
		if changed {
			t.st.running, t.st.pid = running, pid
			if !running {
				t.st.activity = ""
			}
		}
		stop := mustStopForMicrophone(&t.st, microphoneStatus)
		t.mu.Unlock()
		if changed {
			t.render()
		}
		if stop {
			t.refuseDeniedMicrophone()
		}
	}
}

// refuseDeniedMicrophone stops the daemon this app started and says why. The
// answer to the first prompt arrives after the daemon is already running, so
// the check before starting cannot catch a "Don't Allow".
func (t *trayApp) refuseDeniedMicrophone() {
	t.update(func(s *state) { s.fail("Microphone access is off", fixMicrophone) })
	if err := control.Stop(control.PIDPath()); err != nil {
		log.Printf("stopping daemon without microphone access: %v", err)
	}
}

// recentEntries returns the newest notes for the Recent menu. They come from
// the notes folder rather than the event log, so a deleted note drops out and
// a rotated log loses nothing.
func recentEntries() []*browse.Note {
	entries, err := browse.List(time.Time{})
	if err != nil {
		log.Printf("listing notes: %v", err)
		return nil
	}
	return entries[:min(len(entries), recentLimit)]
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
	s.recent = append([]*browse.Note(nil), t.st.recent...)
	t.mu.Unlock()

	menu := t.app.NewMenu()
	menu.Add(s.statusLine()).SetEnabled(false)
	if s.lastErr != "" {
		menu.Add(s.lastErr).SetEnabled(false)
		if s.lastFix == fixMicrophone {
			menu.Add("Open Microphone Settings…").OnClick(func(*application.Context) { t.onboarding.OpenPrivacySettings("Microphone") })
		}
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
			path := e.FilePath
			menu.Add(entryLabel(e)).OnClick(func(*application.Context) { go t.knowledge.showNote(path) })
		}
	}

	menu.AddSeparator()
	menu.Add("Browse Notes…").OnClick(func(*application.Context) { go t.knowledge.show() })
	menu.Add("Settings…").OnClick(func(*application.Context) { go t.settings.show() })
	menu.Add("Set Up Tacit…").OnClick(func(*application.Context) { go t.onboarding.show() })
	menu.Add("Open Knowledge Folder").OnClick(func(*application.Context) { openPath(configure.Dir()) })
	menu.Add("Open Daemon Log").OnClick(func(*application.Context) { openPath(daemonLogPath()) })
	menu.AddSeparator()
	if s.upToDate {
		menu.Add("Tacit " + version + " (up to date)").SetEnabled(false)
	} else {
		menu.Add("Tacit " + version).SetEnabled(false)
	}
	if isRelease(version) { // development builds don't update
		if s.updateFailed {
			menu.Add("Update failed (open log)").OnClick(func(*application.Context) { openPath(updateLogPath()) })
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

	t.setIcon(iconFile(s.icon(), 0))
	t.tray.SetMenu(menu)
}

func (t *trayApp) setIcon(file string) {
	icon, err := trayIcons.ReadFile(file)
	if err != nil {
		log.Printf("tray icon: %v", err)
		return
	}
	t.tray.SetTemplateIcon(icon)
}

// animationInterval is the time between frames of an animated icon.
const animationInterval = 180 * time.Millisecond

// animate steps the icon through its frames while the state has an animated
// one. Only the image changes: redrawing the menu at this rate would be wasteful.
func (t *trayApp) animate(ctx context.Context) {
	tick := time.NewTicker(animationInterval)
	defer tick.Stop()
	for n := 1; ; n++ {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		t.mu.Lock()
		name := t.st.icon()
		t.mu.Unlock()
		if iconFrames[name] == 0 {
			continue
		}
		t.setIcon(iconFile(name, n))
		// The state may have changed, and been drawn, while this frame was
		// being set; draw again so a stale frame does not stay.
		t.mu.Lock()
		stale := t.st.icon() != name
		t.mu.Unlock()
		if stale {
			t.render()
		}
	}
}

// start starts the daemon. A failure is shown in the menu and returned.
//
// The daemon loads its speech model, about ten seconds, before it first opens
// the microphone, and macOS asks for permission only at that moment. So when
// the answer is still open, ask first: the prompt appears at once and the
// model loads while the user answers.
func (t *trayApp) start() error {
	switch microphoneStatus() {
	case permUndetermined:
		requestMicrophone()
	case permDenied:
		// macOS delivers silence rather than an error, so the daemon would
		// start, load its model, and hear nothing.
		t.update(func(s *state) { s.fail("Microphone access is off", fixMicrophone) })
		return errors.New("microphone access denied")
	case permRestricted:
		t.update(func(s *state) { s.fail("Microphone is restricted on this Mac", fixNone) })
		return errors.New("microphone access restricted")
	}
	cli, err := cliPath()
	if err == nil {
		var cmd *exec.Cmd
		if cmd, err = spawnListen(cli, daemonLogPath()); err == nil {
			if err := recordOwned(cmd.Process.Pid); err != nil {
				log.Printf("recording the daemon as the app's: %v", err)
			}
			t.update(func(s *state) { s.ownPID = cmd.Process.Pid; s.clearFailure() })
			go t.reap(cmd)
			return nil
		}
	}
	t.update(func(s *state) { s.fail("Couldn't start: "+err.Error(), fixNone) })
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
			s.fail(explainExit(lastFailure(daemonLogPath())))
		}
	})
}

func (t *trayApp) stop() {
	if err := control.Stop(control.PIDPath()); err != nil {
		t.update(func(s *state) { s.fail("Couldn't stop: "+err.Error(), fixNone) })
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
		if err := control.Stop(control.PIDPath()); err != nil {
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

// checkForUpdate looks for a release newer than this build. The result is
// shown in a dialog only when the user asked: a menu click closes the menu,
// so the menu alone would answer nothing until it was opened again.
func (t *trayApp) checkForUpdate(ctx context.Context, asked bool) {
	if asked {
		t.update(func(s *state) { s.checking = true })
	}
	tag, err := latestRelease(ctx)
	t.update(func(s *state) {
		s.checking = false
		if err != nil {
			return
		}
		if newer(tag, version) {
			s.latest, s.upToDate = tag, false
		} else {
			s.latest, s.upToDate = "", true
		}
	})
	if err != nil {
		log.Printf("checking for updates: %v", err)
	}
	if !asked {
		return
	}
	switch {
	case err != nil:
		t.app.Dialog.Error().SetTitle("Check for Updates").
			SetMessage("Couldn't check for updates: " + err.Error()).Show()
	case newer(tag, version):
		d := t.app.Dialog.Question().SetTitle("Update Available").
			SetMessage(fmt.Sprintf("Tacit %s is available (you have %s). Update now? Tacit will restart.", tag, version))
		update := d.AddButton("Update")
		update.OnClick(func() { go t.upgrade() })
		later := d.AddButton("Later")
		d.SetDefaultButton(update).SetCancelButton(later).Show()
	default:
		t.app.Dialog.Info().SetTitle("Check for Updates").
			SetMessage("Tacit " + version + " is the latest version.").Show()
	}
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
	fail := func(err error) { t.update(func(s *state) { s.fail("Couldn't update: "+err.Error(), fixNone) }) }

	if running && !own {
		fail(errTerminalDaemon)
		return
	}
	if own {
		if err := control.StopAndWait(context.Background(), control.PIDPath(), restartTimeout); err != nil {
			fail(fmt.Errorf("%w; start it from the menu once it has", err))
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
	return filepath.Join(configure.Dir(), "daemon.log")
}

func openPath(p string) {
	if err := exec.Command("open", p).Start(); err != nil {
		log.Printf("open %s: %v", p, err)
	}
}
