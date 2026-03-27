//go:build !darwin

package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

// DecodeFile reads an audio file and returns 16kHz mono float32 PCM samples
// using ffmpeg for decoding. This is the fallback for non-macOS platforms.
func DecodeFile(path string) ([]float32, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("audio file not found: %w", err)
	}

	cmd := exec.Command("ffmpeg",
		"-i", absPath,
		"-ar", fmt.Sprintf("%d", SampleRate),
		"-ac", "1",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-loglevel", "error",
		"pipe:1",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg decode failed: %w", err)
	}

	if len(output) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output")
	}

	numSamples := len(output) / 2
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(output[i*2 : i*2+2]))
		samples[i] = float32(sample) / float32(math.MaxInt16)
	}

	return samples, nil
}
