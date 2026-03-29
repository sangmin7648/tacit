package capture

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"github.com/sangmin7648/tacit/pkg/audio"
)

const Channels = 1

// Mic captures audio from the default microphone at 16kHz mono.
type Mic struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	mu      sync.Mutex
	started bool
}

// New creates a new microphone capture instance.
func New() (*Mic, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("init audio context: %w", err)
	}
	return &Mic{ctx: ctx}, nil
}

// Start begins capturing audio. Each callback delivers a chunk of int16 samples.
// The callback is invoked from the audio thread; keep it fast.
func (m *Mic) Start(onSamples func([]int16)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("already started")
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = Channels
	deviceConfig.SampleRate = audio.SampleRate

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, pInput []byte, frameCount uint32) {
			count := int(frameCount) * Channels
			samples := make([]int16, count)
			for i := 0; i < count; i++ {
				samples[i] = int16(binary.LittleEndian.Uint16(pInput[i*2 : i*2+2]))
			}
			onSamples(samples)
		},
	}

	device, err := malgo.InitDevice(m.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("init capture device: %w", err)
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("start capture: %w", err)
	}

	m.device = device
	m.started = true
	log.Printf("Microphone capture started: %dHz mono", audio.SampleRate)
	return nil
}

// Stop halts audio capture.
func (m *Mic) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.device.Stop()
	m.device.Uninit()
	m.started = false
	log.Printf("Microphone capture stopped")
}

// Close releases all resources.
func (m *Mic) Close() {
	m.Stop()
	if m.ctx != nil {
		_ = m.ctx.Uninit()
		m.ctx.Free()
	}
}

// Stream starts capture and sends int16 sample chunks to the returned channel.
// The channel is closed when ctx is cancelled.
func (m *Mic) Stream(ctx context.Context) (<-chan []int16, error) {
	ch := make(chan []int16, 64)
	var stopped atomic.Bool

	err := m.Start(func(samples []int16) {
		if stopped.Load() {
			return
		}
		select {
		case ch <- samples:
		default:
		}
	})
	if err != nil {
		close(ch)
		return nil, err
	}

	go func() {
		<-ctx.Done()
		stopped.Store(true)
		m.Stop()
		close(ch)
	}()

	return ch, nil
}
