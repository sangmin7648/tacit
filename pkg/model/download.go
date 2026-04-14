package model

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const baseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

// EnsureModel checks if the model file exists at modelPath.
// If not, it downloads the model from HuggingFace (whisper.cpp base URL).
func EnsureModel(modelPath string) error {
	modelFile := filepath.Base(modelPath)
	return EnsureModelFromURL(modelPath, baseURL+"/"+modelFile)
}

// EnsureModelFromURL checks if the model file exists at modelPath.
// If not, it downloads from the given URL.
func EnsureModelFromURL(modelPath, url string) error {
	if _, err := os.Stat(modelPath); err == nil {
		return nil // already exists
	}

	modelFile := filepath.Base(modelPath)

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}

	fmt.Printf("Downloading %s...\n", modelFile)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Write to temp file first, then rename (atomic)
	tmpPath := modelPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	// Wrap reader with progress reporting
	reader := io.Reader(resp.Body)
	if resp.ContentLength > 0 {
		reader = &progressReader{
			reader:   resp.Body,
			total:    resp.ContentLength,
			filename: modelFile,
		}
	}

	written, err := io.Copy(f, reader)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write model file: %w", err)
	}

	if written == 0 {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded empty model file")
	}

	if err := os.Rename(tmpPath, modelPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("finalize model file: %w", err)
	}

	fmt.Printf("\nDownloaded %s (%.1f MB)\n", modelFile, float64(written)/1024/1024)
	return nil
}

// progressReader wraps an io.Reader and prints download progress.
type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	filename string
	lastPct  int
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	pct := int(float64(pr.current) / float64(pr.total) * 100)
	if pct != pr.lastPct && pct%5 == 0 {
		fmt.Printf("\rDownloading %s... %.1f / %.1f MB (%d%%)",
			pr.filename,
			float64(pr.current)/1024/1024,
			float64(pr.total)/1024/1024,
			pct)
		pr.lastPct = pct
	}

	return n, err
}
