package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/sangmin7648/tacit/pkg/audio"
	"github.com/sangmin7648/tacit/pkg/capture"
	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/model"
	"github.com/sangmin7648/tacit/pkg/process"
	"github.com/sangmin7648/tacit/pkg/stt"
	"github.com/sangmin7648/tacit/pkg/storage"
	"github.com/sangmin7648/tacit/pkg/vad"
)

// ErrSkipped is returned by ProcessFile when the audio content is classified
// as meaningless and intentionally not stored. It is not a processing error.
var ErrSkipped = errors.New("content classified as meaningless, skipping")

// Pipeline orchestrates the VAD→STT→Process→Store flow.
type Pipeline struct {
	cfg        *config.Config
	whisper    *stt.Whisper
	whisperMu  sync.Mutex // serialises concurrent STT calls from multiple sources
	classifier process.Classifier
	baseDir    string
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

	classifier := process.NewClassifier(cfg)
	if p, ok := classifier.(process.Pinger); ok {
		if err := p.Ping(context.Background()); err != nil {
			w.Close() // Release whisper to avoid ggml Metal cleanup crash on exit
			return nil, err
		}
	}

	return &Pipeline{
		cfg:        cfg,
		whisper:    w,
		classifier: classifier,
		baseDir:    baseDir,
	}, nil
}

// Close releases pipeline resources.
func (p *Pipeline) Close() {
	if p.whisper != nil {
		p.whisper.Close()
	}
}

// classifyItem holds STT text waiting for async classification.
type classifyItem struct {
	text      string
	timestamp time.Time
}

// Run starts one or more audio sources through the VAD→STT→classify→store
// loop.  Each source runs in its own goroutine; STT calls are serialised by a
// mutex so the shared Whisper instance is used safely.
// It blocks until ctx is cancelled or all sources exit.
func (p *Pipeline) Run(ctx context.Context, sources []capture.AudioSource) error {
	if len(sources) == 0 {
		return fmt.Errorf("no audio sources configured")
	}

	// Single shared classify channel / worker so batching still works across
	// multiple capture sources.
	classifyCh := make(chan classifyItem, 64)
	var classifyWg sync.WaitGroup
	classifyWg.Add(1)
	go func() {
		defer classifyWg.Done()
		p.classifyLoop(ctx, classifyCh)
	}()

	// Start one VAD+STT goroutine per source.
	var sourceWg sync.WaitGroup
	for i, src := range sources {
		sourceWg.Add(1)
		label := sourceLabel(i, len(sources))
		go func(src capture.AudioSource, label string) {
			defer sourceWg.Done()
			if err := p.runSource(ctx, src, label, classifyCh); err != nil {
				log.Printf("[%s] source error: %v", label, err)
			}
		}(src, label)
	}

	sourceWg.Wait()
	close(classifyCh)
	classifyWg.Wait()
	return nil
}

// sourceLabel returns a short display name for the nth source.
func sourceLabel(i, total int) string {
	if total == 1 {
		return "mic"
	}
	switch i {
	case 0:
		return "mic"
	case 1:
		return "speaker"
	default:
		return fmt.Sprintf("src%d", i)
	}
}

// runSource runs a single audio source through VAD→STT and enqueues results
// onto classifyCh.  It returns when ctx is cancelled or the source stream
// closes.
func (p *Pipeline) runSource(ctx context.Context, src capture.AudioSource, label string, classifyCh chan<- classifyItem) error {
	// Init per-source VAD (256 samples = 16 ms at 16 kHz).
	const hopSize = 256
	v, err := vad.New(hopSize, float32(p.cfg.SpeechThreshold))
	if err != nil {
		return fmt.Errorf("init vad: %w", err)
	}
	defer v.Close()

	stream, err := src.Stream(ctx)
	if err != nil {
		return fmt.Errorf("start stream: %w", err)
	}

	segBuf := audio.NewSegmentBuffer(audio.SampleRate, p.cfg.MinSpeechDur)
	var frameBuf []int16
	silenceFrames := 0
	silenceLimit := int(p.cfg.SilenceDuration.Seconds() * float64(audio.SampleRate) / float64(hopSize))

	log.Printf("[%s] listening (silence=%v, minSpeech=%v)", label, p.cfg.SilenceDuration, p.cfg.MinSpeechDur)

	for chunk := range stream {
		frameBuf = append(frameBuf, chunk...)

		processed := 0
		for processed+hopSize <= len(frameBuf) {
			frame := frameBuf[processed : processed+hopSize]
			processed += hopSize

			_, isSpeech, err := v.Process(frame)
			if err != nil {
				log.Printf("[%s] VAD error: %v", label, err)
				continue
			}

			// Energy gate.
			if isSpeech && p.cfg.EnergyThreshold > 0 {
				var sum float64
				for _, s := range frame {
					sum += float64(s) * float64(s)
				}
				rms := math.Sqrt(sum / float64(len(frame)))
				if rms < p.cfg.EnergyThreshold {
					isSpeech = false
				}
			}

			if isSpeech {
				silenceFrames = 0
				if !segBuf.IsActive() {
					segBuf.Start()
					log.Printf("[%s] speech started", label)
				}
				segBuf.Append(audio.Int16ToFloat32(frame))
			} else if segBuf.IsActive() {
				segBuf.Append(audio.Int16ToFloat32(frame))
				silenceFrames++

				if silenceFrames >= silenceLimit {
					log.Printf("[%s] speech ended (%.1fs)", label, segBuf.Duration().Seconds())
					seg, ok := segBuf.Finish()
					silenceFrames = 0
					if ok {
						p.transcribeAndQueue(ctx, seg, label, classifyCh)
					} else {
						log.Printf("[%s] segment too short, discarding", label)
					}
				}
			}
		}
		// Compact: move unprocessed samples to front to prevent memory leak.
		n := copy(frameBuf, frameBuf[processed:])
		frameBuf = frameBuf[:n]
	}

	return nil
}

// transcribeAndQueue runs STT (serialised across sources) then queues text
// for async classification.
func (p *Pipeline) transcribeAndQueue(ctx context.Context, seg *audio.AudioSegment, label string, ch chan<- classifyItem) {
	log.Printf("[%s] transcribing %.1fs of audio", label, seg.Duration.Seconds())

	p.whisperMu.Lock()
	text, err := p.whisper.Transcribe(ctx, seg.Samples, p.cfg.InitialPrompt)
	p.whisperMu.Unlock()

	if err != nil {
		log.Printf("[%s] STT error: %v", label, err)
		return
	}
	if text == "" {
		log.Printf("[%s] STT produced empty text, skipping", label)
		return
	}
	log.Printf("[%s] STT: %s", label, text)

	ch <- classifyItem{text: text, timestamp: time.Now()}
}

// classifyLoop processes classify items from the channel, batching when
// multiple items are queued (e.g. during a long classification call).
func (p *Pipeline) classifyLoop(ctx context.Context, ch <-chan classifyItem) {
	for {
		item, ok := <-ch
		if !ok {
			return
		}

		batch := []classifyItem{item}
	drain:
		for {
			select {
			case more, ok := <-ch:
				if !ok {
					break drain
				}
				batch = append(batch, more)
			default:
				break drain
			}
		}

		existingCategories := storage.ListCategories(p.baseDir)

		if len(batch) == 1 {
			log.Printf("Classifying 1 segment...")
			classified, err := p.classifier.Classify(ctx, batch[0].text, existingCategories)
			if err != nil {
				log.Printf("Classify error: %v", err)
				continue
			}
			if classified.Skip {
				log.Printf("Skipping meaningless segment")
				continue
			}
			p.storeEntry(classified, batch[0])
		} else {
			log.Printf("Batch classifying %d segments in one CLI call...", len(batch))
			texts := make([]string, len(batch))
			for i, b := range batch {
				texts[i] = b.text
			}
			results, err := p.classifier.ClassifyBatch(ctx, texts, existingCategories)
			if err != nil {
				log.Printf("Batch classify error, falling back to individual: %v", err)
				for _, b := range batch {
					classified, err := p.classifier.Classify(ctx, b.text, existingCategories)
					if err != nil {
						log.Printf("Classify error: %v", err)
						continue
					}
					if classified.Skip {
						log.Printf("Skipping meaningless segment")
						continue
					}
					p.storeEntry(classified, b)
				}
				continue
			}
			for i, classified := range results {
				if i < len(batch) {
					if classified.Skip {
						log.Printf("Skipping meaningless segment %d", i+1)
						continue
					}
					p.storeEntry(classified, batch[i])
				}
			}
		}
	}
}

// storeEntry saves a classified item as a knowledge entry.
func (p *Pipeline) storeEntry(classified *process.ClassifyResult, item classifyItem) {
	entry := newKnowledgeEntry(classified, item.text, item.timestamp)
	filePath, err := storage.Write(p.baseDir, entry)
	if err != nil {
		log.Printf("Write error: %v", err)
		return
	}
	log.Printf("Knowledge entry saved: %s", filePath)
}

func newKnowledgeEntry(classified *process.ClassifyResult, content string, ts time.Time) *storage.KnowledgeEntry {
	return &storage.KnowledgeEntry{
		Title:     classified.Title,
		Category:  classified.Category,
		CreatedAt: ts,
		Summary:   classified.Summary,
		Content:   content,
	}
}

// ProcessFile processes an audio file through the full pipeline:
// decode → STT → classify → save as markdown knowledge entry.
// Returns the path to the created knowledge file.
func (p *Pipeline) ProcessFile(ctx context.Context, audioPath string) (string, error) {
	log.Printf("Decoding audio file: %s", audioPath)
	samples, err := audio.DecodeFile(audioPath)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	duration := audio.DurationFromSamples(len(samples), audio.SampleRate)
	log.Printf("Decoded %d samples (%.2f seconds)", len(samples), duration.Seconds())

	if duration < p.cfg.MinSpeechDur {
		return "", fmt.Errorf("audio too short: %v (minimum: %v)", duration, p.cfg.MinSpeechDur)
	}

	log.Printf("Running STT...")
	p.whisperMu.Lock()
	text, err := p.whisper.Transcribe(ctx, samples, p.cfg.InitialPrompt)
	p.whisperMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("transcribe: %w", err)
	}
	if text == "" {
		return "", fmt.Errorf("STT produced empty text")
	}
	log.Printf("STT result: %s", text)

	log.Printf("Classifying with LLM...")
	classifyStart := time.Now()
	existingCategories := storage.ListCategories(p.baseDir)
	classified, err := p.classifier.Classify(ctx, text, existingCategories)
	if err != nil {
		return "", fmt.Errorf("classify: %w", err)
	}
	log.Printf("Classified in %.1fs: title=%q, category=%q", time.Since(classifyStart).Seconds(), classified.Title, classified.Category)

	if classified.Skip {
		return "", ErrSkipped
	}

	entry := newKnowledgeEntry(classified, text, time.Now())

	filePath, err := storage.Write(p.baseDir, entry)
	if err != nil {
		return "", fmt.Errorf("write knowledge entry: %w", err)
	}
	log.Printf("Saved knowledge entry: %s", filePath)

	return filePath, nil
}
