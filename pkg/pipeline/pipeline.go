package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rapportlabs/sttdb/pkg/audio"
	"github.com/rapportlabs/sttdb/pkg/config"
	"github.com/rapportlabs/sttdb/pkg/process"
	"github.com/rapportlabs/sttdb/pkg/stt"
	"github.com/rapportlabs/sttdb/pkg/storage"
)

// Pipeline orchestrates the VAD→STT→Process→Store flow.
type Pipeline struct {
	cfg     *config.Config
	whisper *stt.Whisper
	baseDir string
}

// New creates a new pipeline with the given configuration.
func New(cfg *config.Config) (*Pipeline, error) {
	baseDir := config.BaseDir()

	modelPath := config.ModelPath(cfg.WhisperModel)
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("whisper model not found at %s: %w", modelPath, err)
	}

	w, err := stt.NewWhisper(modelPath)
	if err != nil {
		return nil, fmt.Errorf("init whisper: %w", err)
	}

	return &Pipeline{
		cfg:     cfg,
		whisper: w,
		baseDir: baseDir,
	}, nil
}

// Close releases pipeline resources.
func (p *Pipeline) Close() {
	if p.whisper != nil {
		p.whisper.Close()
	}
}

// ProcessFile processes an audio file through the full pipeline:
// decode → STT → classify → save as markdown knowledge entry.
// Returns the path to the created knowledge file.
func (p *Pipeline) ProcessFile(ctx context.Context, audioPath string) (string, error) {
	// 1. Decode audio file to PCM
	log.Printf("Decoding audio file: %s", audioPath)
	samples, err := audio.DecodeFile(audioPath)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	log.Printf("Decoded %d samples (%.2f seconds)", len(samples), float64(len(samples))/16000.0)

	// Check minimum duration
	duration := time.Duration(float64(len(samples)) / 16000.0 * float64(time.Second))
	if duration < p.cfg.MinSpeechDur {
		return "", fmt.Errorf("audio too short: %v (minimum: %v)", duration, p.cfg.MinSpeechDur)
	}

	// 2. STT
	log.Printf("Running STT...")
	text, err := p.whisper.Transcribe(ctx, samples)
	if err != nil {
		return "", fmt.Errorf("transcribe: %w", err)
	}
	if text == "" {
		return "", fmt.Errorf("STT produced empty text")
	}
	log.Printf("STT result: %s", text)

	// 3. Classify with Claude Code CLI
	log.Printf("Classifying with Claude Code CLI...")
	existingCategories := listExistingCategories(p.baseDir)
	classified, err := process.Classify(ctx, text, existingCategories, p.cfg.ClaudeModel)
	if err != nil {
		return "", fmt.Errorf("classify: %w", err)
	}
	log.Printf("Classified: title=%q, category=%q", classified.Title, classified.Category)

	// 4. Save as markdown
	entry := &storage.KnowledgeEntry{
		Title:     classified.Title,
		Category:  classified.Category,
		CreatedAt: time.Now(),
		Summary:   classified.Summary,
		Content:   text,
	}

	filePath, err := storage.Write(p.baseDir, entry)
	if err != nil {
		return "", fmt.Errorf("write knowledge entry: %w", err)
	}
	log.Printf("Saved knowledge entry: %s", filePath)

	return filePath, nil
}

// listExistingCategories scans the knowledge base directory for existing categories.
func listExistingCategories(baseDir string) []string {
	var categories []string
	seen := make(map[string]bool)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden dirs and config files
		if strings.HasPrefix(name, ".") || name == "models" {
			continue
		}
		categories = append(categories, name)
		seen[name] = true

		// Check for subcategories
		subPath := filepath.Join(baseDir, name)
		subEntries, err := os.ReadDir(subPath)
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if subEntry.IsDir() {
				subCat := name + "/" + subEntry.Name()
				if !seen[subCat] {
					categories = append(categories, subCat)
					seen[subCat] = true
				}
			}
		}
	}

	return categories
}
