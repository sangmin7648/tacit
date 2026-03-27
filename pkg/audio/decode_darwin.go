//go:build darwin

package audio

/*
#cgo LDFLAGS: -framework AudioToolbox -framework CoreFoundation
#include <AudioToolbox/ExtendedAudioFile.h>
#include <stdlib.h>
#include <string.h>

// decode_audio decodes an audio file to 16kHz mono float32 PCM using macOS AudioToolbox.
// Supports all formats macOS can decode: m4a, mp3, wav, flac, aiff, alac, etc.
// Caller must free *out_samples with free().
static int decode_audio(const char* path, float** out_samples, int* out_count) {
    CFURLRef url = CFURLCreateFromFileSystemRepresentation(
        kCFAllocatorDefault, (const UInt8*)path, strlen(path), false);
    if (!url) return -1;

    ExtAudioFileRef audioFile = NULL;
    OSStatus err = ExtAudioFileOpenURL(url, &audioFile);
    CFRelease(url);
    if (err != noErr) return (int)err;

    // Get input format to calculate resampled frame count
    AudioStreamBasicDescription inputFormat;
    UInt32 propSize = sizeof(inputFormat);
    err = ExtAudioFileGetProperty(audioFile, kExtAudioFileProperty_FileDataFormat, &propSize, &inputFormat);
    if (err != noErr) { ExtAudioFileDispose(audioFile); return (int)err; }

    // Get total frame count in source format
    SInt64 fileLengthFrames = 0;
    propSize = sizeof(fileLengthFrames);
    err = ExtAudioFileGetProperty(audioFile, kExtAudioFileProperty_FileLengthFrames, &propSize, &fileLengthFrames);
    if (err != noErr) { ExtAudioFileDispose(audioFile); return (int)err; }

    // Set output format: 16kHz mono float32 PCM
    AudioStreamBasicDescription outputFormat;
    memset(&outputFormat, 0, sizeof(outputFormat));
    outputFormat.mSampleRate       = 16000.0;
    outputFormat.mFormatID         = kAudioFormatLinearPCM;
    outputFormat.mFormatFlags      = kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked;
    outputFormat.mBitsPerChannel   = 32;
    outputFormat.mChannelsPerFrame = 1;
    outputFormat.mFramesPerPacket  = 1;
    outputFormat.mBytesPerFrame    = 4;
    outputFormat.mBytesPerPacket   = 4;

    err = ExtAudioFileSetProperty(audioFile, kExtAudioFileProperty_ClientDataFormat, sizeof(outputFormat), &outputFormat);
    if (err != noErr) { ExtAudioFileDispose(audioFile); return (int)err; }

    // Estimate output frame count after resampling (add margin)
    SInt64 estimatedFrames = (SInt64)((double)fileLengthFrames * 16000.0 / inputFormat.mSampleRate) + 4096;

    float* samples = (float*)malloc((size_t)estimatedFrames * sizeof(float));
    if (!samples) { ExtAudioFileDispose(audioFile); return -2; }

    // Read in chunks
    SInt64 totalRead = 0;
    UInt32 chunkSize = 8192;
    while (totalRead < estimatedFrames) {
        UInt32 framesToRead = chunkSize;
        if (totalRead + framesToRead > estimatedFrames) {
            framesToRead = (UInt32)(estimatedFrames - totalRead);
        }

        AudioBufferList bufferList;
        bufferList.mNumberBuffers = 1;
        bufferList.mBuffers[0].mNumberChannels = 1;
        bufferList.mBuffers[0].mDataByteSize = framesToRead * sizeof(float);
        bufferList.mBuffers[0].mData = samples + totalRead;

        err = ExtAudioFileRead(audioFile, &framesToRead, &bufferList);
        if (err != noErr) {
            free(samples);
            ExtAudioFileDispose(audioFile);
            return (int)err;
        }
        if (framesToRead == 0) break; // EOF
        totalRead += framesToRead;
    }

    ExtAudioFileDispose(audioFile);

    *out_samples = samples;
    *out_count = (int)totalRead;
    return 0;
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

const targetSampleRate = 16000

// DecodeFile reads an audio file and returns 16kHz mono float32 PCM samples
// using macOS AudioToolbox. Supports m4a, mp3, wav, flac, aiff, alac, etc.
func DecodeFile(path string) ([]float32, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("audio file not found: %w", err)
	}

	cPath := C.CString(absPath)
	defer C.free(unsafe.Pointer(cPath))

	var cSamples *C.float
	var cCount C.int

	ret := C.decode_audio(cPath, &cSamples, &cCount)
	if ret != 0 {
		return nil, fmt.Errorf("AudioToolbox decode failed (OSStatus %d)", int(ret))
	}
	defer C.free(unsafe.Pointer(cSamples))

	count := int(cCount)
	if count == 0 {
		return nil, fmt.Errorf("AudioToolbox produced no samples")
	}

	// Copy C samples to Go slice
	samples := make([]float32, count)
	cSlice := unsafe.Slice((*float32)(unsafe.Pointer(cSamples)), count)
	copy(samples, cSlice)

	return samples, nil
}
