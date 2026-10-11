package speechdetector

import (
	"log"
	"math"
	"time"

	micrecorder "github.com/sangmin7648/tacit/core/internal/components/mic-recorder"
)

// hopSize is the VAD frame: 256 samples is 16 ms at 16 kHz.
const hopSize = 256

// preRollFrames is how much audio from just before speech onset is kept, about
// 192 ms. VAD often fires a frame or two late, and the clipped onset was a
// common cause of a mis-transcribed first word.
const preRollFrames = 12

// Options tune a Detector.
type Options struct {
	// SpeechThreshold is the VAD confidence, 0 to 1, above which a frame is speech.
	SpeechThreshold float32
	// EnergyThreshold is the RMS below which a frame VAD called speech is
	// treated as silence anyway; 0 turns the gate off.
	EnergyThreshold float64
	// MinSpeech is the shortest segment worth keeping.
	MinSpeech time.Duration
	// Silence is how long speech must stop before the segment ends.
	Silence time.Duration
	// Split, when positive, cuts ongoing speech into segments of this length.
	Split time.Duration
}

// Kind names what Feed observed.
type Kind int

const (
	// SpeechStarted: VAD heard speech after silence.
	SpeechStarted Kind = iota
	// SegmentSplit: ongoing speech reached Options.Split and was cut; speech
	// continues into a new segment.
	SegmentSplit
	// SpeechEnded: silence lasted Options.Silence and closed the segment.
	SpeechEnded
)

// Event is one thing Feed observed.
type Event struct {
	Kind Kind
	// Segment is the finished audio for SegmentSplit and SpeechEnded, or nil
	// when it was shorter than Options.MinSpeech.
	Segment *AudioSegment
	// Duration is the length of the audio that just finished, kept or not.
	Duration time.Duration
}

// Detector turns a PCM stream into speech segments. It is not safe for
// concurrent use; give each stream its own.
type Detector struct {
	opts          Options
	vad           *VAD
	segment       *SegmentBuffer
	pending       []int16 // samples not yet a whole VAD frame
	silenceFrames int
	silenceLimit  int
	preRoll       []float32
}

// NewDetector creates a Detector. Close it when done.
func NewDetector(opts Options) (*Detector, error) {
	v, err := newVAD(hopSize, opts.SpeechThreshold)
	if err != nil {
		return nil, err
	}
	return &Detector{
		opts:         opts,
		vad:          v,
		segment:      NewSegmentBuffer(micrecorder.SampleRate, opts.MinSpeech, opts.Split),
		silenceLimit: int(opts.Silence.Seconds() * float64(micrecorder.SampleRate) / float64(hopSize)),
	}, nil
}

// Close releases the VAD.
func (d *Detector) Close() { d.vad.Close() }

// Feed consumes a chunk of PCM and returns what it observed, in order.
func (d *Detector) Feed(chunk []int16) []Event {
	var events []Event
	d.pending = append(d.pending, chunk...)
	processed := 0
	for processed+hopSize <= len(d.pending) {
		frame := d.pending[processed : processed+hopSize]
		processed += hopSize
		events = d.frame(frame, events)
	}
	// Keep only the unprocessed tail, so the buffer does not grow forever.
	n := copy(d.pending, d.pending[processed:])
	d.pending = d.pending[:n]
	return events
}

func (d *Detector) frame(frame []int16, events []Event) []Event {
	isSpeech, ok := d.isSpeech(frame)
	if !ok {
		return events // a frame VAD could not judge is dropped, not counted as silence
	}
	samples := micrecorder.Int16ToFloat32(frame)

	switch {
	case isSpeech:
		d.silenceFrames = 0
		if !d.segment.IsActive() {
			d.segment.Start()
			if len(d.preRoll) > 0 {
				d.segment.Append(d.preRoll)
				d.preRoll = d.preRoll[:0]
			}
			events = append(events, Event{Kind: SpeechStarted})
		}
		d.segment.Append(samples)
		if d.opts.Split > 0 && d.segment.Duration() >= d.opts.Split {
			dur := d.segment.Duration()
			seg, _ := d.segment.Finish()
			events = append(events, Event{Kind: SegmentSplit, Segment: seg, Duration: dur})
			d.segment.Start() // speech is still going on
		}

	case d.segment.IsActive():
		d.segment.Append(samples)
		d.silenceFrames++
		if d.silenceFrames >= d.silenceLimit {
			dur := d.segment.Duration()
			seg, _ := d.segment.Finish()
			d.silenceFrames = 0
			events = append(events, Event{Kind: SpeechEnded, Segment: seg, Duration: dur})
		}

	default:
		d.preRoll = append(d.preRoll, samples...)
		if over := len(d.preRoll) - preRollFrames*hopSize; over > 0 {
			d.preRoll = d.preRoll[:copy(d.preRoll, d.preRoll[over:])]
		}
	}
	return events
}

// isSpeech asks VAD about frame, then vetoes speech too quiet to be anyone
// talking nearby.
func (d *Detector) isSpeech(frame []int16) (speech, ok bool) {
	_, speech, err := d.vad.Process(frame)
	if err != nil {
		log.Printf("VAD error: %v", err)
		return false, false
	}
	if !speech || d.opts.EnergyThreshold <= 0 {
		return speech, true
	}
	var sum float64
	for _, s := range frame {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum/float64(len(frame))) >= d.opts.EnergyThreshold, true
}
