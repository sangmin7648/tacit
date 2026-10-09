package statusreporter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// DefaultMaxBytes caps the live event log before it is rotated. An always-on
// daemon writes to this file for weeks, so it needs a ceiling; a few megabytes
// still holds far more history than a front end displays.
const DefaultMaxBytes int64 = 5 << 20

// Writer appends events to a file as newline-delimited JSON.
//
// The daemon writes this file whoever started it, so a front end can watch a
// daemon it did not spawn — the case that rules out handing events to a child
// process's stdout. One rotated generation is kept alongside the live file.
//
// A Writer is safe for concurrent use.
type Writer struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	f        *os.File
	size     int64
	// err holds the first write failure. Later writes keep being attempted:
	// losing the event log must never take the daemon down with it.
	err error
}

// NewWriter opens path for appending, creating its parent directory. A
// maxBytes of zero uses DefaultMaxBytes; negative disables rotation.
func NewWriter(path string, maxBytes int64) (*Writer, error) {
	if maxBytes == 0 {
		maxBytes = DefaultMaxBytes
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating event log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening event log: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat event log: %w", err)
	}
	return &Writer{path: path, maxBytes: maxBytes, f: f, size: info.Size()}, nil
}

// Observe appends e to the log. It satisfies Observer.
//
// A failure is recorded and returned by Err, not propagated: the pipeline calls
// this from the audio goroutines, and a full disk should cost the user their
// event history, not their transcription.
func (w *Writer) Observe(e Event) {
	line, err := json.Marshal(e)
	if err != nil {
		w.mu.Lock()
		w.setErr(fmt.Errorf("marshalling event: %w", err))
		w.mu.Unlock()
		return
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return // closed
	}
	if w.maxBytes > 0 && w.size+int64(len(line)) > w.maxBytes {
		w.rotateLocked()
	}
	n, err := w.f.Write(line)
	w.size += int64(n)
	if err != nil {
		w.setErr(fmt.Errorf("writing event: %w", err))
	}
}

// rotateLocked moves the live log aside and starts a new one. On failure it
// leaves the current file in place and keeps appending: an oversized log beats
// a lost one. Callers must hold w.mu.
func (w *Writer) rotateLocked() {
	if err := w.f.Close(); err != nil {
		w.setErr(fmt.Errorf("closing event log for rotation: %w", err))
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		w.setErr(fmt.Errorf("rotating event log: %w", err))
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		w.setErr(fmt.Errorf("reopening event log after rotation: %w", err))
		w.f = nil
		return
	}
	w.f = f
	w.size = 0
}

// setErr records the first failure only. Callers must hold w.mu.
func (w *Writer) setErr(err error) {
	if w.err == nil {
		w.err = err
	}
}

// Err returns the first write failure, if any.
func (w *Writer) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// Close closes the underlying file. Observe is a no-op afterwards.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// Decode reads newline-delimited events from r.
//
// A line that does not parse is skipped rather than failing the whole read: the
// log's last line is truncated whenever the daemon is killed mid-write, and a
// front end should still get the history in front of it.
func Decode(r io.Reader) ([]Event, error) {
	var out []Event
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return out, fmt.Errorf("reading event log: %w", err)
	}
	return out, nil
}

// ReadFile decodes the events in the log at path. A missing file reads as no
// events, since the daemon may simply never have run.
func ReadFile(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening event log: %w", err)
	}
	defer f.Close()
	return Decode(f)
}
