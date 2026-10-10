package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sangmin7648/tacit/core/workflows/configure"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"v0.12.0", "v0.11.0", true},
		{"v0.11.1", "v0.11.0", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.10.0", "v0.9.0", true}, // numerically, not as text
		{"v0.11.0", "v0.11.0", false},
		{"v0.11.0", "v0.12.0", false},
		{"v0.12.0", "v0.11.0-16-gfe768d8", false}, // a dev build is not compared
		{"v0.12.0", "dev", false},
	} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	if isRelease("v0.11.0-16-gfe768d8") || isRelease("dev") || !isRelease("v0.11.0") {
		t.Error("isRelease misclassifies a build")
	}
}

func TestLatestRelease(t *testing.T) {
	location := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if location != "" {
			w.Header().Set("Location", location)
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer srv.Close()
	orig := latestURL
	latestURL = srv.URL
	t.Cleanup(func() { latestURL = orig })

	location = "https://github.com/sangmin7648/tacit/releases/tag/v0.12.0"
	if tag, err := latestRelease(context.Background()); err != nil || tag != "v0.12.0" {
		t.Errorf("latestRelease = %q, %v; want v0.12.0", tag, err)
	}
	for _, loc := range []string{"", "https://github.com/sangmin7648/tacit/releases/tag/nightly"} {
		location = loc
		if tag, err := latestRelease(context.Background()); err == nil {
			t.Errorf("Location %q gave %q, want an error", loc, tag)
		}
	}
}

// runUpdater runs updaterScript as spawnUpdater would, but in the foreground,
// for an app that has already exited, with `open` recorded instead of run.
// install is the install.sh it downloads ("" for a download that fails).
func runUpdater(t *testing.T, install string, installedExists bool) (status, opened string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	// The app "starts" on the second open, as when LaunchServices ignores the first.
	os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/sh\necho \"$@\" > \""+dir+"/opened\"\n"+
		"[ -e \""+dir+"/tried\" ] && touch \""+dir+"/running\"\ntouch \""+dir+"/tried\"\n"), 0o755)
	os.WriteFile(filepath.Join(bin, "pgrep"), []byte("#!/bin/sh\n[ -e \""+dir+"/running\" ]\n"), 0o755)

	url := "file://" + filepath.Join(dir, "missing.sh")
	if install != "" {
		os.WriteFile(filepath.Join(dir, "install.sh"), []byte(install), 0o644)
		url = "file://" + filepath.Join(dir, "install.sh")
	}
	installed := filepath.Join(dir, "Applications", "Tacit.app")
	if installedExists {
		os.MkdirAll(installed, 0o755)
	}

	exited := exec.Command("true")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(dir, "status")
	cmd := exec.Command("/bin/sh", "-c", updaterScript, "tacit-updater",
		strconv.Itoa(exited.Process.Pid), installed, "/build/Tacit.app", resumeFlag)
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin",
		"TACIT_INSTALL_URL="+url, "TACIT_OPEN_POLL=0.01", "TACIT_UPDATE_STATUS="+statusPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("updater: %v\n%s", err, out)
	}
	s, _ := os.ReadFile(statusPath)
	o, _ := os.ReadFile(filepath.Join(dir, "opened"))
	return strings.TrimSpace(string(s)), strings.TrimSpace(strings.ReplaceAll(string(o), dir, ""))
}

func TestUpdaterScript(t *testing.T) {
	if status, opened := runUpdater(t, "exit 0\n", true); status != "0" || opened != "/Applications/Tacit.app --args "+resumeFlag {
		t.Errorf("success: status %q, opened %q", status, opened)
	}
	if status, _ := runUpdater(t, "exit 3\n", true); status != "3" {
		t.Errorf("install.sh failing: status %q, want its exit code 3", status)
	}
	// A failed download must not run an empty script that "succeeds".
	if status, _ := runUpdater(t, "", true); status != "1" {
		t.Errorf("download failing: status %q, want 1", status)
	}
	// Nothing installed (a first install that failed): reopen the app that ran.
	if _, opened := runUpdater(t, "exit 1\n", false); opened != "/build/Tacit.app --args "+resumeFlag {
		t.Errorf("fallback: opened %q", opened)
	}
}

func TestLastUpdateFailed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(configure.Dir(), 0o755)
	if lastUpdateFailed() {
		t.Error("no status reported a failure")
	}
	os.WriteFile(updateStatusPath(), []byte("0\n"), 0o644)
	if lastUpdateFailed() {
		t.Error("status 0 reported a failure")
	}
	os.WriteFile(updateStatusPath(), []byte("1\n"), 0o644)
	if !lastUpdateFailed() {
		t.Error("status 1 not reported")
	}
	if lastUpdateFailed() {
		t.Error("a failure was reported twice")
	}
}
