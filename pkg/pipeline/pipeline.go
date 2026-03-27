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
	"github.com/rapportlabs/sttdb/pkg/capture"
	"github.com/rapportlabs/sttdb/pkg/config"
	"github.com/rapportlabs/sttdb/pkg/model"
	"github.com/rapportlabs/sttdb/pkg/process"
	"github.com/rapportlabs/sttdb/pkg/stt"
	"github.com/rapportlabs/sttdb/pkg/storage"
	"github.com/rapportlabs/sttdb/pkg/vad"
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
	if err := model.EnsureModel(modelPath); err != nil {
		return nil, fmt.Errorf("ensure whisper model: %w", err)
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

// Run starts the real-time capture→VAD→STT→classify→store loop.
// It blocks until ctx is cancelled.
func (p *Pipeline) Run(ctx context.Context) error {
	// Init VAD (256 samples = 16ms at 16kHz)
	const hopSize = 256
	v, err := vad.New(hopSize, float32(p.cfg.SpeechThreshold))
	if err != nil {
		return fmt.Errorf("init vad: %w", err)
	}
	defer v.Close()
	log.Printf("VAD initialized (hop=%d, threshold=%.2f)", hopSize, p.cfg.SpeechThreshold)

	// Init microphone capture
	mic, err := capture.New()
	if err != nil {
		return fmt.Errorf("init capture: %w", err)
	}
	defer mic.Close()

	stream, err := mic.Stream(ctx)
	if err != nil {
		return fmt.Errorf("start capture stream: %w", err)
	}

	// Segment buffer for accumulating speech audio
	segBuf := audio.NewSegmentBuffer(capture.SampleRate, p.cfg.MinSpeechDur)

	// VAD frame buffer: accumulate samples until we have hopSize
	var frameBuf []int16
	silenceFrames := 0
	silenceLimit := int(p.cfg.SilenceDuration.Seconds() * float64(capture.SampleRate) / float64(hopSize))

	log.Printf("Listening for speech... (silence timeout: %v, min speech: %v)", p.cfg.SilenceDuration, p.cfg.MinSpeechDur)

	for chunk := range stream {
		frameBuf = append(frameBuf, chunk...)

		// Process all complete frames in the buffer
		for len(frameBuf) >= hopSize {
			frame := frameBuf[:hopSize]
			frameBuf = frameBuf[hopSize:]

			_, isSpeech, err := v.Process(frame)
			if err != nil {
				log.Printf("VAD error: %v", err)
				continue
			}

			if isSpeech {
				silenceFrames = 0
				if !segBuf.IsActive() {
					segBuf.Start()
					log.Printf("Speech started")
				}
				segBuf.Append(capture.Int16ToFloat32(frame))
			} else if segBuf.IsActive() {
				// Still append during silence gap (captures trailing audio)
				segBuf.Append(capture.Int16ToFloat32(frame))
				silenceFrames++

				if silenceFrames >= silenceLimit {
					log.Printf("Speech ended (%.1fs)", segBuf.Duration().Seconds())
					seg, ok := segBuf.Finish()
					silenceFrames = 0
					if ok {
						p.processSegment(ctx, seg)
					} else {
						log.Printf("Segment too short, discarding")
					}
				}
			}
		}
	}

	return nil
}

// processSegment runs STT→classify→store on a speech segment.
func (p *Pipeline) processSegment(ctx context.Context, seg *audio.AudioSegment) {
	log.Printf("Processing segment: %.1fs of audio", seg.Duration.Seconds())

	text, err := p.whisper.Transcribe(ctx, seg.Samples)
	if err != nil {
		log.Printf("STT error: %v", err)
		return
	}
	if text == "" {
		log.Printf("STT produced empty text, skipping")
		return
	}
	log.Printf("STT: %s", text)

	existingCategories := listExistingCategories(p.baseDir)
	classified, err := process.Classify(ctx, text, existingCategories, p.cfg.ClaudeModel)
	if err != nil {
		log.Printf("Classify error: %v", err)
		return
	}

	entry := &storage.KnowledgeEntry{
		Title:     classified.Title,
		Category:  classified.Category,
		CreatedAt: time.Now(),
		Summary:   classified.Summary,
		Content:   text,
	}

	filePath, err := storage.Write(p.baseDir, entry)
	if err != nil {
		log.Printf("Write error: %v", err)
		return
	}
	log.Printf("Knowledge entry saved: %s", filePath)
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
