package statusreporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// Follow calls fn with each event appended to the log at path from the moment
// it is called, until ctx is done. Events already in the log are not replayed;
// use ReadFile for history.
//
// The log is written by another process, so Follow polls every interval. It
// picks up a log that does not exist yet, and on rotation drains the old file
// to its end before moving to the new one, so events written around a
// rotation are not lost. fn runs on Follow's goroutine.
func Follow(ctx context.Context, path string, interval time.Duration, fn func(Event)) error {
	f, err := os.Open(path)
	switch {
	case err == nil:
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			f.Close()
			return err
		}
	case errors.Is(err, os.ErrNotExist):
		f = nil
	default:
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	var partial []byte
	buf := make([]byte, 64<<10)
	drain := func() {
		for {
			n, err := f.Read(buf)
			if n > 0 {
				partial = emitLines(append(partial, buf[:n]...), fn)
			}
			if n == 0 || err != nil {
				return
			}
		}
	}

	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if f != nil {
			drain()
		}
		// A different file at path means the log was rotated (or appeared). The
		// old file has just been drained, so switch and read the new one from
		// its start without waiting for the next tick.
		if onDisk, err := os.Stat(path); err == nil {
			var current os.FileInfo
			if f != nil {
				current, _ = f.Stat()
			}
			if current == nil || !os.SameFile(current, onDisk) {
				if next, err := os.Open(path); err == nil {
					if f != nil {
						f.Close()
					}
					f, partial = next, nil
					continue
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// emitLines hands every complete line in data to fn and returns the trailing
// incomplete line, if any, to be completed by the next read.
func emitLines(data []byte, fn func(Event)) []byte {
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return data
		}
		line := data[:i]
		data = data[i+1:]
		if len(line) == 0 {
			continue
		}
		var e Event
		if json.Unmarshal(line, &e) == nil {
			fn(e)
		}
	}
}
