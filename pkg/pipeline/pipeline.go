package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"strings"

	"github.com/sangmin7648/tacit/pkg/audio"
	"github.com/sangmin7648/tacit/pkg/capture"
	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/model"
	"github.com/sangmin7648/tacit/pkg/process"
	"github.com/sangmin7648/tacit/pkg/storage"
	"github.com/sangmin7648/tacit/pkg/stt"
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
// labels[i] is the display name for sources[i] used in log messages.
// It blocks until ctx is cancelled or all sources exit.
func (p *Pipeline) Run(ctx context.Context, sources []capture.AudioSource, labels []string) error {
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
		label := labels[i]
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

// retryDelay is the wait before restarting a source after its stream ends or
// re-initialisation fails.  stallTimeout is how long runSourceOnce waits for an
// audio chunk before assuming the stream has silently died (e.g. SCStream torn
// down by macOS on sleep without firing didStopWithError:).  Both are vars, not
// consts, so tests can shrink them.
var (
	retryDelay   = 5 * time.Second
	stallTimeout = 15 * time.Second
)

// runSource runs a single audio source through VAD→STT and enqueues results
// onto classifyCh.  It restarts the source automatically whenever a capture
// session ends for any reason other than ctx cancellation — whether the stream
// closed unexpectedly (SCStream stopped by macOS), stalled with no audio, or
// re-initialisation failed transiently (common right after sleep/wake).  It
// returns only when ctx is cancelled.
func (p *Pipeline) runSource(ctx context.Context, src capture.AudioSource, label string, classifyCh chan<- classifyItem) error {
	for {
		err := p.runSourceOnce(ctx, src, label, classifyCh)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			log.Printf("[%s] capture session error: %v; restarting in %v", label, err, retryDelay)
		} else {
			log.Printf("[%s] stream ended, restarting in %v", label, retryDelay)
		}
		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return nil
		}
	}
}

// runSourceOnce runs one capture session for a source.  It returns when ctx is
// cancelled or the source's stream channel is closed (normal or unexpected).
func (p *Pipeline) runSourceOnce(ctx context.Context, src capture.AudioSource, label string, classifyCh chan<- classifyItem) error {
	// Init per-source VAD (256 samples = 16 ms at 16 kHz).
	const hopSize = 256
	v, err := vad.New(hopSize, float32(p.cfg.SpeechThreshold))
	if err != nil {
		return fmt.Errorf("init vad: %w", err)
	}
	defer v.Close()

	stream, err := src.Stream(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil // ctx cancelled during restart, not a real error
		}
		return fmt.Errorf("start stream: %w", err)
	}

	minSpeechDur := p.cfg.MinSpeechDur
	silenceDuration := p.cfg.SilenceDuration
	maxSegmentDur := p.cfg.MaxSegmentDur

	if label == "mic" {
		if p.cfg.MicMinSpeechDur != 0 {
			minSpeechDur = p.cfg.MicMinSpeechDur
		}
		if p.cfg.MicSilenceDuration != 0 {
			silenceDuration = p.cfg.MicSilenceDuration
		}
		if p.cfg.MicMaxSegmentDur != 0 {
			maxSegmentDur = p.cfg.MicMaxSegmentDur
		}
	} else if label == "speaker" {
		if p.cfg.SpeakerMinSpeechDur != 0 {
			minSpeechDur = p.cfg.SpeakerMinSpeechDur
		}
		if p.cfg.SpeakerSilenceDuration != 0 {
			silenceDuration = p.cfg.SpeakerSilenceDuration
		}
		if p.cfg.SpeakerMaxSegmentDur != 0 {
			maxSegmentDur = p.cfg.SpeakerMaxSegmentDur
		}
	}

	segBuf := audio.NewSegmentBuffer(audio.SampleRate, minSpeechDur, maxSegmentDur)
	var frameBuf []int16
	silenceFrames := 0
	silenceLimit := int(silenceDuration.Seconds() * float64(audio.SampleRate) / float64(hopSize))

	// preRoll keeps a short window of the most recent pre-speech audio so the
	// onset of a phrase isn't clipped when VAD fires a frame or two late — a
	// common cause of mis-transcribed first words. Experimental-only.
	const preRollFrames = 12 // ~192ms at 16ms/frame
	const preRollMax = preRollFrames * hopSize
	var preRoll []float32

	// textBuf accumulates STT results from split chunks within one speech session.
	// All chunks are joined and sent as a single classify item when silence is
	// detected, or earlier once the session passes maxSessionDur: an
	// uninterrupted meeting never fires speech-ended, so without a cap it piles
	// minutes of speech into one item and a single classification failure takes
	// the whole thing down with it.
	var textBuf []string
	var sessionStart time.Time
	maxSessionDur := p.cfg.MaxSessionDur

	// flush hands the accumulated transcript to the classifier and starts a new
	// session. Filler-only text is dropped here rather than by the LLM, so the
	// classifier is only ever asked about text that has something in it.
	flush := func(reason string) {
		if len(textBuf) == 0 {
			return
		}
		text := strings.Join(textBuf, " ")
		textBuf = textBuf[:0]
		start := sessionStart
		sessionStart = time.Now()
		if process.IsFiller(text) {
			log.Printf("[%s] transcript is filler only, discarding: %s", label, process.TruncateForLog(text))
			return
		}
		log.Printf("[%s] flushing transcript for classification (%s)", label, reason)
		classifyCh <- classifyItem{text: text, timestamp: start}
	}

	log.Printf("[%s] listening (silence=%v, minSpeech=%v)", label, silenceDuration, minSpeechDur)

	// Inactivity watchdog: a live capture stream (mic or SCStream) delivers PCM
	// chunks continuously, even during silence, so a prolonged absence of chunks
	// means the stream has died without closing its channel.  When that happens
	// we return so runSource restarts the source.
	stall := time.NewTimer(stallTimeout)
	defer stall.Stop()

	for {
		var chunk []int16
		select {
		case c, ok := <-stream:
			if !ok {
				return nil // channel closed (normal or unexpected stop) → restart
			}
			chunk = c
			if !stall.Stop() {
				select {
				case <-stall.C:
				default:
				}
			}
			stall.Reset(stallTimeout)
		case <-stall.C:
			log.Printf("[%s] no audio for %v, assuming stream stalled; restarting", label, stallTimeout)
			return nil
		case <-ctx.Done():
			return nil
		}

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
					if len(textBuf) == 0 {
						sessionStart = time.Now()
					}
					segBuf.Start()
					// Prepend buffered pre-speech audio to recover a clipped onset.
					if p.cfg.Experimental && len(preRoll) > 0 {
						segBuf.Append(preRoll)
						preRoll = preRoll[:0]
					}
					log.Printf("[%s] speech started", label)
				}
				segBuf.Append(audio.Int16ToFloat32(frame))

				// Force-split long segments to cap memory usage; accumulate
				// the resulting text to merge into one file at session end.
				if maxSegmentDur > 0 && segBuf.Duration() >= maxSegmentDur {
					log.Printf("[%s] segment capped at %.1fs, splitting", label, segBuf.Duration().Seconds())
					seg, ok := segBuf.Finish()
					if ok {
						if text := p.transcribeSync(ctx, seg, label); text != "" {
							textBuf = append(textBuf, text)
						}
					}
					if maxSessionDur > 0 && !sessionStart.IsZero() && time.Since(sessionStart) >= maxSessionDur {
						flush("session cap reached")
					}
					segBuf.Start() // speech is still ongoing; restart immediately
				}
			} else if segBuf.IsActive() {
				segBuf.Append(audio.Int16ToFloat32(frame))
				silenceFrames++

				if silenceFrames >= silenceLimit {
					log.Printf("[%s] speech ended (%.1fs)", label, segBuf.Duration().Seconds())
					seg, ok := segBuf.Finish()
					silenceFrames = 0
					if ok {
						if text := p.transcribeSync(ctx, seg, label); text != "" {
							textBuf = append(textBuf, text)
						}
					} else if len(textBuf) == 0 {
						log.Printf("[%s] segment too short, discarding", label)
					}
					// Flush accumulated text as a single classify item.
					flush("speech ended")
				}
			} else if p.cfg.Experimental {
				// Leading silence: keep the most recent frames as pre-roll so a
				// late-firing VAD onset doesn't clip the first word.
				preRoll = append(preRoll, audio.Int16ToFloat32(frame)...)
				if over := len(preRoll) - preRollMax; over > 0 {
					preRoll = preRoll[:copy(preRoll, preRoll[over:])]
				}
			}
		}
		// Compact: move unprocessed samples to front to prevent memory leak.
		n := copy(frameBuf, frameBuf[processed:])
		frameBuf = frameBuf[:n]
	}
}

// sttOptions builds the per-call whisper options from the pipeline config.
func (p *Pipeline) sttOptions() stt.Options {
	return stt.Options{
		Language:      p.cfg.Language,
		InitialPrompt: p.cfg.InitialPrompt,
		Experimental:  p.cfg.Experimental,
	}
}

// transcribeSync runs STT (serialised across sources) and returns the
// transcribed text, or "" on error or empty result.
func (p *Pipeline) transcribeSync(ctx context.Context, seg *audio.AudioSegment, label string) string {
	log.Printf("[%s] transcribing %.1fs of audio", label, seg.Duration.Seconds())

	p.whisperMu.Lock()
	text, err := p.whisper.Transcribe(ctx, seg.Samples, p.sttOptions())
	p.whisperMu.Unlock()

	if err != nil {
		log.Printf("[%s] STT error: %v", label, err)
		return ""
	}
	if text == "" {
		// whisper discards a window outright when it reads as non-speech
		// (no_speech_prob above threshold together with a low average logprob),
		// so an empty result is a decode-level rejection rather than a failure.
		log.Printf("[%s] STT rejected %.1fs as non-speech, nothing transcribed", label, seg.Duration.Seconds())
		return ""
	}
	log.Printf("[%s] STT: %s", label, text)

	filtered := process.FilterHallucinations(text, p.cfg.TranscriptDenylist)
	switch {
	case filtered == "":
		log.Printf("[%s] transcript was entirely hallucinated boilerplate, discarding", label)
	case filtered != text:
		log.Printf("[%s] stripped hallucinated boilerplate, kept: %s", label, process.TruncateForLog(filtered))
	}
	return filtered
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
			p.classifyAndStore(ctx, batch[0], existingCategories)
			continue
		}

		log.Printf("Batch classifying %d segments in one CLI call...", len(batch))
		texts := make([]string, len(batch))
		for i, b := range batch {
			texts[i] = b.text
		}
		results, err := p.classifier.ClassifyBatch(ctx, texts, existingCategories)
		if err != nil {
			log.Printf("Batch classify error, falling back to individual: %v", err)
			results = nil
		} else if len(results) != len(batch) {
			log.Printf("Batch returned %d results for %d segments; the remainder fall back to individual classification", len(results), len(batch))
		}

		// Walk the batch, not the results: ranging over a short response left the
		// trailing segments unvisited and discarded them without a trace.
		for i, b := range batch {
			if i < len(results) && results[i] != nil {
				p.dispatch(b, results[i])
				continue
			}
			p.classifyAndStore(ctx, b, existingCategories)
		}
	}
}

// classifyAndStore classifies a single item, retrying once, and stores the
// outcome. A transcript is never discarded because classification failed: if
// the classifier errors out or answers with nothing usable, the entry is stored
// unclassified so it still turns up in `tacit list`.
func (p *Pipeline) classifyAndStore(ctx context.Context, item classifyItem, existingCategories []string) {
	classified, err := p.classifier.Classify(ctx, item.text, existingCategories)
	if err != nil && ctx.Err() == nil {
		log.Printf("Classify error (%v); retrying once", err)
		classified, err = p.classifier.Classify(ctx, item.text, existingCategories)
	}
	if err != nil {
		log.Printf("Classify failed (%v); storing unclassified: %s", err, process.TruncateForLog(item.text))
		p.storeEntry(process.FallbackResult(item.text), item)
		return
	}
	p.dispatch(item, classified)
}

// dispatch stores an already-classified item, substituting a fallback entry
// when the model asked for neither a skip nor supplied anything to store.
func (p *Pipeline) dispatch(item classifyItem, classified *process.ClassifyResult) {
	switch {
	case classified.Skip:
		// Log the text: a skip is the one path that intentionally throws speech
		// away, and until now it left nothing behind to check that against.
		log.Printf("Skipping segment the classifier called meaningless: %s", process.TruncateForLog(item.text))
	case !classified.Usable():
		log.Printf("Classifier returned no content fields; storing unclassified: %s", process.TruncateForLog(item.text))
		p.storeEntry(process.FallbackResult(item.text), item)
	default:
		p.storeEntry(classified, item)
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
		Keywords:  classified.Keywords,
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
	text, err := p.whisper.Transcribe(ctx, samples, p.sttOptions())
	p.whisperMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("transcribe: %w", err)
	}
	if text == "" {
		return "", fmt.Errorf("STT produced empty text")
	}
	log.Printf("STT result: %s", text)

	text = process.FilterHallucinations(text, p.cfg.TranscriptDenylist)
	if text == "" {
		log.Printf("Transcript was entirely hallucinated boilerplate")
		return "", ErrSkipped
	}
	if process.IsFiller(text) {
		log.Printf("Transcript is filler only")
		return "", ErrSkipped
	}

	log.Printf("Classifying with LLM...")
	classifyStart := time.Now()
	existingCategories := storage.ListCategories(p.baseDir)
	classified, err := p.classifier.Classify(ctx, text, existingCategories)
	if err != nil {
		log.Printf("Classify error (%v); retrying once", err)
		classified, err = p.classifier.Classify(ctx, text, existingCategories)
	}
	switch {
	case err != nil:
		// Storing the transcript unclassified beats returning an error and
		// leaving a successful transcription with nowhere to go.
		log.Printf("Classify failed (%v); storing unclassified", err)
		classified = process.FallbackResult(text)
	case classified.Skip:
		return "", ErrSkipped
	case !classified.Usable():
		log.Printf("Classifier returned no content fields; storing unclassified")
		classified = process.FallbackResult(text)
	default:
		log.Printf("Classified in %.1fs: title=%q, category=%q", time.Since(classifyStart).Seconds(), classified.Title, classified.Category)
	}

	entry := newKnowledgeEntry(classified, text, time.Now())

	filePath, err := storage.Write(p.baseDir, entry)
	if err != nil {
		return "", fmt.Errorf("write knowledge entry: %w", err)
	}
	log.Printf("Saved knowledge entry: %s", filePath)

	return filePath, nil
}
