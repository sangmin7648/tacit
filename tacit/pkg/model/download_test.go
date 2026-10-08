package model

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
)

func serve(t *testing.T, body []byte, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestDownload_ReportsProgressToCompletion(t *testing.T) {
	body := bytes.Repeat([]byte("w"), 256<<10)
	srv, _ := serve(t, body, http.StatusOK)
	path := filepath.Join(t.TempDir(), "models", "ggml-test.bin")

	var calls int
	var last, total int64
	written, err := download(context.Background(), path, srv.URL+"/ggml-test.bin", func(done, tot int64) {
		if done < last {
			t.Errorf("progress went backwards: %d after %d", done, last)
		}
		calls++
		last, total = done, tot
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if written != int64(len(body)) {
		t.Errorf("written = %d, want %d", written, len(body))
	}
	if calls == 0 {
		t.Fatal("progress was never reported")
	}
	if last != int64(len(body)) || total != int64(len(body)) {
		t.Errorf("final progress = %d/%d, want %d/%d", last, total, len(body), len(body))
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("model file not written intact (err %v)", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file left behind")
	}
}

// A user closing the onboarding window mid-download must not leave a partial
// file that a later run would mistake for a complete model.
func TestDownload_CancelLeavesNoFile(t *testing.T) {
	body := bytes.Repeat([]byte("w"), 4<<20)
	srv, _ := serve(t, body, http.StatusOK)
	path := filepath.Join(t.TempDir(), "ggml-test.bin")

	ctx, cancel := context.WithCancel(context.Background())
	_, err := download(ctx, path, srv.URL+"/ggml-test.bin", func(done, total int64) {
		if done > 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	for _, p := range []string{path, path + ".tmp"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s exists after a cancelled download", p)
		}
	}
}

func TestDownload_HTTPErrorLeavesNoFile(t *testing.T) {
	srv, _ := serve(t, []byte("not found"), http.StatusNotFound)
	path := filepath.Join(t.TempDir(), "ggml-test.bin")

	if _, err := download(context.Background(), path, srv.URL+"/x", nil); err == nil {
		t.Fatal("download succeeded on HTTP 404")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("model file exists after a failed download")
	}
}

// An existing model is never re-fetched, by either entry point.
func TestEnsureAndDownload_SkipExistingModel(t *testing.T) {
	srv, hits := serve(t, []byte("new"), http.StatusOK)
	path := filepath.Join(t.TempDir(), "ggml-test.bin")
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureModelFromURL(path, srv.URL+"/x"); err != nil {
		t.Fatalf("EnsureModelFromURL: %v", err)
	}
	if err := Download(context.Background(), path, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server hit %d times for a model already on disk", n)
	}
	if got, _ := os.ReadFile(path); string(got) != "existing" {
		t.Errorf("existing model overwritten: %q", got)
	}
}

// The CLI's printer keeps the output it had before progress became a callback:
// one redraw per 5%, nothing when the length is unknown.
func TestPrinter_Every5Percent(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	p := &printer{filename: "ggml-test.bin"}
	for done := int64(0); done <= 100; done++ {
		p.report(done, 100)
	}
	p.report(50, -1)
	w.Close()
	os.Stdout = stdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if n := bytes.Count(buf.Bytes(), []byte("\rDownloading ggml-test.bin...")); n != 20 {
		t.Errorf("printed %d progress lines, want 20 (5%%..100%%)", n)
	}
}
