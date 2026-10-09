//go:build e2e

package pipeline

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sangmin7648/tacit/pkg/audio"
	"github.com/sangmin7648/tacit/pkg/capture"
	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/events"
	"github.com/sangmin7648/tacit/pkg/storage"
)

// TestE2E plays a recording through the same path a live microphone takes —
// VAD, STT, classification, storage — with the real whisper model and the
// classifier from the user's config. Only the knowledge base is redirected, to
// a temp dir, so the run leaves no entry behind in ~/.tacit.
func TestE2E(t *testing.T) {
	samples := readWAV(t, "testdata/test_voice_recording.wav")

	cfg, err := config.LoadWithOverride(config.ConfigPath(), config.OverridePath())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	defer p.Close()
	p.baseDir = t.TempDir()
	rec := &recorder{}
	p.SetObserver(rec)

	// Trailing silence long enough for VAD to call the speech finished.
	silence := make([]int16, int((cfg.SilenceDuration+time.Second).Seconds()*audio.SampleRate))
	src := &fakeSource{streamFn: func(ctx context.Context, call int) (<-chan []int16, error) {
		return playback(ctx, append(samples, silence...)), nil
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	done := make(chan struct{})
	go func() {
		p.Run(ctx, []capture.AudioSource{src}, []string{"mic"})
		close(done)
	}()

	for rec.count(events.KindStored) == 0 {
		if ctx.Err() != nil {
			t.Fatalf("no entry stored before timeout; events: %v", rec.kinds())
		}
		time.Sleep(200 * time.Millisecond)
	}
	cancel()
	<-done

	entries, err := storage.ListEntries(p.baseDir, time.Time{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("want one entry in the temp knowledge base, got %d (err %v)", len(entries), err)
	}
	body, err := os.ReadFile(entries[0].FilePath)
	if err != nil {
		t.Fatalf("read stored entry: %v", err)
	}
	t.Logf("stored %s:\n%s", entries[0].FilePath, body)
}

// playback streams samples in mic-sized chunks, then holds the stream open
// until ctx ends, as a live microphone would.
func playback(ctx context.Context, samples []int16) <-chan []int16 {
	const chunk = 1024
	ch := make(chan []int16)
	go func() {
		defer close(ch)
		for i := 0; i < len(samples); i += chunk {
			select {
			case ch <- samples[i:min(i+chunk, len(samples))]:
			case <-ctx.Done():
				return
			}
		}
		<-ctx.Done()
	}()
	return ch
}

// readWAV reads a canonical 16 kHz mono 16-bit PCM WAV file. The recording is
// kept uncompressed so this needs no decoder.
func readWAV(t *testing.T, path string) []int16 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkWAV(data); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	pcm := data[44:]
	out := make([]int16, len(pcm)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(pcm[2*i:]))
	}
	return out
}

func checkWAV(d []byte) error {
	if len(d) < 44 || string(d[0:4]) != "RIFF" || string(d[8:12]) != "WAVE" ||
		string(d[12:16]) != "fmt " || string(d[36:40]) != "data" {
		return fmt.Errorf("not a canonical WAV file")
	}
	format := binary.LittleEndian.Uint16(d[20:])
	channels := binary.LittleEndian.Uint16(d[22:])
	rate := binary.LittleEndian.Uint32(d[24:])
	bits := binary.LittleEndian.Uint16(d[34:])
	if format != 1 || channels != 1 || rate != audio.SampleRate || bits != 16 {
		return fmt.Errorf("want PCM mono %d Hz 16-bit, got format %d, %d ch, %d Hz, %d-bit",
			audio.SampleRate, format, channels, rate, bits)
	}
	return nil
}
