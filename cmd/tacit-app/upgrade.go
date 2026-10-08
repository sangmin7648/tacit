package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sangmin7648/tacit/pkg/config"
)

// The app updates the way `tacit update` does — by running install.sh, which
// replaces Tacit.app and the CLI linked to it together. install.sh refuses to
// run while the app is running, so the app hands it to a detached shell that
// waits for the app to exit, runs it, and reopens the app.

var (
	// latestURL redirects to the newest release's tag page. It is the same
	// redirect install.sh resolves, which unlike the REST API has no rate limit.
	latestURL  = "https://github.com/sangmin7648/tacit/releases/latest"
	installURL = "https://raw.githubusercontent.com/sangmin7648/tacit/main/install.sh"
)

// checkInterval is how often a running app looks for a new release.
const checkInterval = 24 * time.Hour

// resumeFlag, passed when the updater reopens the app, starts listening again
// if the app was listening before the update.
const resumeFlag = "--resume-listening"

var releaseTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// isRelease reports whether v is a release tag (v1.2.3). Development builds
// (v0.11.0-16-gfe768d8, dev) never offer an update.
func isRelease(v string) bool {
	return releaseTag.MatchString(v)
}

// newer reports whether release tag a is newer than release tag b.
func newer(a, b string) bool {
	ma, mb := releaseTag.FindStringSubmatch(a), releaseTag.FindStringSubmatch(b)
	if ma == nil || mb == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(ma[i])
		y, _ := strconv.Atoi(mb[i])
		if x != y {
			return x > y
		}
	}
	return false
}

// latestRelease returns the newest release's tag, read from where latestURL
// redirects.
func latestRelease(ctx context.Context) (string, error) {
	client := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, latestURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	_, tag, ok := strings.Cut(resp.Header.Get("Location"), "/releases/tag/")
	if !ok || !isRelease(tag) {
		return "", fmt.Errorf("no release found (%s)", resp.Status)
	}
	return tag, nil
}

func updateLogPath() string    { return filepath.Join(config.BaseDir(), "update.log") }
func updateStatusPath() string { return filepath.Join(config.BaseDir(), "update-status") }

// updaterScript runs detached from the app: $1 is the app's PID, $2 the app
// install.sh installs, $3 the app running now — reopened instead if $2 is
// missing, as after a failed first install — and $4 the flag to reopen with
// (may be empty). It downloads install.sh before running it, so a failed
// download is a failure rather than an empty script that "succeeds", and
// records install.sh's exit status for the reopened app to report.
const updaterScript = `
while kill -0 "$1" 2>/dev/null; do sleep 0.2; done
script=$(mktemp)
if curl -fsSL "$TACIT_INSTALL_URL" -o "$script"; then
  sh "$script"
  status=$?
else
  status=1
fi
rm -f "$script"
echo "$status" > "$TACIT_UPDATE_STATUS"
app="$2"
[ -d "$app" ] || app="$3"
if [ -n "$4" ]; then open "$app" --args "$4"; else open "$app"; fi
`

// spawnUpdater starts the detached updater for the app with PID appPID,
// running from the bundle at current.
func spawnUpdater(appPID int, current string, resume bool) error {
	installed := filepath.Join(os.Getenv("HOME"), "Applications", "Tacit.app")
	flag := ""
	if resume {
		flag = resumeFlag
	}
	if err := os.MkdirAll(config.BaseDir(), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(updateLogPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	os.Remove(updateStatusPath())

	cmd := exec.Command("/bin/sh", "-c", updaterScript, "tacit-updater", strconv.Itoa(appPID), installed, current, flag)
	cmd.Env = append(os.Environ(), "TACIT_INSTALL_URL="+installURL, "TACIT_UPDATE_STATUS="+updateStatusPath())
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// lastUpdateFailed reports, once, whether the update that reopened the app
// failed; the status file is removed after reading.
func lastUpdateFailed() bool {
	data, err := os.ReadFile(updateStatusPath())
	if err != nil {
		return false
	}
	os.Remove(updateStatusPath())
	return strings.TrimSpace(string(data)) != "0"
}

var errTerminalDaemon = errors.New("tacit is listening from a terminal — stop it with 'tacit stop', then update")
