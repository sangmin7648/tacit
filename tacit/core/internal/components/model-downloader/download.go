package modeldownloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const baseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

// Progress reports a download's progress: done bytes out of total. total is -1
// when the server did not send a length. It is called for every chunk read, so
// a caller that redraws on it should throttle.
type Progress func(done, total int64)

// EnsureModel checks if the model file exists at modelPath.
// If not, it downloads the model from HuggingFace (whisper.cpp base URL),
// printing progress to stdout.
func EnsureModel(modelPath string) error {
	return EnsureModelFromURL(modelPath, modelURL(modelPath))
}

// EnsureModelFromURL checks if the model file exists at modelPath.
// If not, it downloads from the given URL, printing progress to stdout.
func EnsureModelFromURL(modelPath, url string) error {
	if _, err := os.Stat(modelPath); err == nil {
		return nil // already exists
	}
	modelFile := filepath.Base(modelPath)
	fmt.Printf("Downloading %s...\n", modelFile)

	written, err := download(context.Background(), modelPath, url, PrintProgress(modelFile))
	if err != nil {
		return err
	}
	fmt.Printf("\nDownloaded %s (%.1f MB)\n", modelFile, float64(written)/1024/1024)
	return nil
}

// Download fetches the model for modelPath unless it is already there,
// reporting progress to progress (which may be nil) instead of printing. It is
// the entry point for a front end that draws its own progress bar. Cancelling
// ctx aborts the download and leaves no partial file behind.
func Download(ctx context.Context, modelPath string, progress Progress) error {
	if _, err := os.Stat(modelPath); err == nil {
		return nil
	}
	_, err := download(ctx, modelPath, modelURL(modelPath), progress)
	return err
}

func modelURL(modelPath string) string {
	return baseURL + "/" + filepath.Base(modelPath)
}

// download writes url to modelPath via a temp file, renamed into place only
// once complete, and returns the bytes written.
func download(ctx context.Context, modelPath, url string, progress Progress) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		return 0, fmt.Errorf("create model directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("download model: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmpPath := modelPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("create temp file: %w", err)
	}

	reader := io.Reader(resp.Body)
	if progress != nil {
		total := resp.ContentLength
		if total <= 0 {
			total = -1
		}
		reader = &progressReader{reader: resp.Body, total: total, report: progress}
	}

	written, err := io.Copy(f, reader)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("write model file: %w", err)
	}

	if written == 0 {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("downloaded empty model file")
	}

	if err := os.Rename(tmpPath, modelPath); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("finalize model file: %w", err)
	}
	return written, nil
}

// progressReader passes every read's running total to report.
type progressReader struct {
	reader  io.Reader
	total   int64
	current int64
	report  Progress
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	pr.report(pr.current, pr.total)
	return n, err
}

// PrintProgress is the CLI's Progress: one line, redrawn in place every 5%.
func PrintProgress(filename string) Progress {
	p := &printer{filename: filename}
	return p.report
}

type printer struct {
	filename string
	lastPct  int
}

func (p *printer) report(done, total int64) {
	if total <= 0 {
		return
	}
	pct := int(float64(done) / float64(total) * 100)
	if pct != p.lastPct && pct%5 == 0 {
		fmt.Printf("\rDownloading %s... %.1f / %.1f MB (%d%%)",
			p.filename,
			float64(done)/1024/1024,
			float64(total)/1024/1024,
			pct)
		p.lastPct = pct
	}
}
