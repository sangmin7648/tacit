// speaker_darwin.m — Core Audio process-tap system-audio capture for macOS 14.2+.
// Compiled as Objective-C by CGo on Darwin.
//
// Why a tap and not ScreenCaptureKit: SCStream needs an SCContentFilter built
// from a live SCDisplay, so capture died whenever the machine had zero displays
// — every external-monitor drop in clamshell mode killed system audio with
// "failed to find display or window to capture".  A process tap has no display
// dependency at all: it is attached to a private aggregate device that contains
// nothing but the tap itself, so no display and no physical audio device has to
// survive for capture to keep running.
//
// The C contract in speaker_darwin.h is unchanged, so the Go side and the
// pipeline's restart logic are unaffected.

#import <Foundation/Foundation.h>
#import <CoreAudio/CoreAudio.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#import <AudioToolbox/AudioToolbox.h>
#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>

#include "speaker_darwin.h"

#if !defined(__MAC_OS_X_VERSION_MAX_ALLOWED) || __MAC_OS_X_VERSION_MAX_ALLOWED < 140200
#error "tacit needs the macOS 14.2 SDK or newer (Core Audio process taps). Update Xcode / Command Line Tools."
#endif

// One second of 16 kHz output.  An IOProc cycle cannot deliver anywhere near a
// second of input, so a single conversion pass always fits.
#define TACIT_OUT_SCRATCH_FRAMES 16000

// ---------------------------------------------------------------------------
// SpeakerCapture
// ---------------------------------------------------------------------------

struct SpeakerCapture {
    uintptr_t           goHandle;

    AudioObjectID       tapID;        // kAudioObjectUnknown == not created
    AudioObjectID       aggregateID;  // kAudioObjectUnknown == not created
    AudioDeviceIOProcID procID;       // NULL == not created
    AudioConverterRef   converter;    // NULL == not created

    AudioStreamBasicDescription tapFormat;  // from kAudioTapPropertyFormat

    // Pre-allocated conversion target.  The IOProc runs on a real-time thread
    // and must not allocate, so everything it needs is reserved up front.
    int16_t*            outScratch;

    // Per-cycle hand-off from the IOProc to the converter's input callback.
    // Only ever touched from the IOProc thread, so no atomics needed.
    const AudioBufferList* pendingInput;
    UInt32                 pendingFrames;
    bool                   pendingConsumed;

    atomic_bool         stopped;     // IOProc and listener early-out
    atomic_bool         ioTornDown;  // once-guard for stop + destroy IOProc
    atomic_bool         notified;    // once-guard for tacitSpeakerStoppedCallback

    // Listener delivery is pinned to our own serial queue so teardown can put a
    // barrier in front of free(cap) — see speaker_stop.
    void*               notifyQueue;    // (__bridge_retained) dispatch_queue_t
    void*               listenerBlock;  // (__bridge_retained) AudioObjectPropertyListenerBlock
    bool                listenersInstalled;
};

// ---------------------------------------------------------------------------
// AudioConverter input callback
// ---------------------------------------------------------------------------

// Hands the current IOProc cycle's input buffer to the converter exactly once.
// Reporting zero packets afterwards is the "no more input right now" signal, so
// FillComplexBuffer returns with whatever it has produced.
//
// Leave ioData untouched on that path.  Zeroing mNumberBuffers to signal
// emptiness makes FillComplexBuffer fail the whole call with paramErr (-50)
// even though it has already produced good samples.
static OSStatus tacitConverterInputProc(AudioConverterRef inConverter,
                                        UInt32* ioNumberDataPackets,
                                        AudioBufferList* ioData,
                                        AudioStreamPacketDescription** outPacketDesc,
                                        void* inUserData) {
    (void)inConverter;

    SpeakerCapture* cap = (SpeakerCapture*)inUserData;
    if (outPacketDesc) *outPacketDesc = NULL;

    if (cap == NULL || cap->pendingConsumed ||
        cap->pendingInput == NULL || cap->pendingFrames == 0) {
        *ioNumberDataPackets = 0;
        return noErr;
    }

    // Pointer-copy the IOProc's buffer list straight through: zero-copy, and
    // correct for interleaved and non-interleaved alike because a
    // non-interleaved ASBD already states mBytesPerFrame per channel.
    UInt32 n = cap->pendingInput->mNumberBuffers;
    memcpy(ioData, cap->pendingInput,
           sizeof(AudioBufferList) + (n - 1) * sizeof(AudioBuffer));

    *ioNumberDataPackets = cap->pendingFrames;
    cap->pendingConsumed = true;
    return noErr;
}

// ---------------------------------------------------------------------------
// Device IOProc
// ---------------------------------------------------------------------------

// Called on a real-time audio thread for every cycle of the aggregate device,
// including during silence — which is what keeps the pipeline's stall watchdog
// from mistaking a quiet room for a dead stream.
static OSStatus tacitSpeakerIOProc(AudioObjectID inDevice,
                                   const AudioTimeStamp* inNow,
                                   const AudioBufferList* inInputData,
                                   const AudioTimeStamp* inInputTime,
                                   AudioBufferList* outOutputData,
                                   const AudioTimeStamp* inOutputTime,
                                   void* inClientData) {
    (void)inDevice;
    (void)inNow;
    (void)inInputTime;
    (void)outOutputData;
    (void)inOutputTime;

    SpeakerCapture* cap = (SpeakerCapture*)inClientData;
    if (cap == NULL) return noErr;
    if (atomic_load_explicit(&cap->stopped, memory_order_acquire)) return noErr;
    if (inInputData == NULL || inInputData->mNumberBuffers == 0) return noErr;

    const AudioBuffer* src = &inInputData->mBuffers[0];
    if (src->mData == NULL || src->mDataByteSize == 0) return noErr;

    UInt32 bytesPerFrame = cap->tapFormat.mBytesPerFrame;
    if (bytesPerFrame == 0) return noErr;

    UInt32 inFrames = src->mDataByteSize / bytesPerFrame;
    if (inFrames == 0) return noErr;

    cap->pendingInput    = inInputData;
    cap->pendingFrames   = inFrames;
    cap->pendingConsumed = false;

    // Ask for the exact resampled frame count plus slack for the converter's
    // filter delay, clamped to the scratch buffer.
    UInt32 outFrames = (UInt32)((double)inFrames * 16000.0 / cap->tapFormat.mSampleRate) + 64;
    if (outFrames > TACIT_OUT_SCRATCH_FRAMES) outFrames = TACIT_OUT_SCRATCH_FRAMES;

    AudioBufferList outList;
    outList.mNumberBuffers              = 1;
    outList.mBuffers[0].mNumberChannels = 1;
    outList.mBuffers[0].mDataByteSize   = outFrames * (UInt32)sizeof(int16_t);
    outList.mBuffers[0].mData           = cap->outScratch;

    OSStatus err = AudioConverterFillComplexBuffer(
        cap->converter, tacitConverterInputProc, cap, &outFrames, &outList, NULL);

    cap->pendingInput = NULL;

    if (err != noErr || outFrames == 0) return noErr;

    // Re-check under the latch: after AudioDeviceDestroyIOProcID returns, the Go
    // side is free to delete the cgo handle, and calling into a deleted handle
    // panics the Go runtime.
    if (!atomic_load_explicit(&cap->stopped, memory_order_acquire)) {
        tacitSpeakerSamplesCallback(cap->goHandle, cap->outScratch, (int)outFrames);
    }
    return noErr;
}

// ---------------------------------------------------------------------------
// IO teardown + unexpected-stop notification
// ---------------------------------------------------------------------------

// Stops the device and destroys the IOProc.  Once AudioDeviceDestroyIOProcID
// returns, the HAL guarantees the IOProc will not be invoked again — which is
// what makes it safe for Go to delete the cgo handle afterwards.  Idempotent,
// so the listener path and speaker_stop can both call it.
static void tacitSpeakerTeardownIO(SpeakerCapture* cap) {
    if (atomic_exchange(&cap->ioTornDown, true)) return;

    atomic_store_explicit(&cap->stopped, true, memory_order_release);

    if (cap->procID != NULL && cap->aggregateID != kAudioObjectUnknown) {
        AudioDeviceStop(cap->aggregateID, cap->procID);
        AudioDeviceDestroyIOProcID(cap->aggregateID, cap->procID);
        cap->procID = NULL;
    }
}

// Stands in for SCStreamDelegate's didStopWithError:.  Deliberately does NOT
// destroy the tap or the aggregate device — that stays speaker_stop's job, run
// from Go's teardown() on the next Stream()/Close()/ctx-cancel.  This mirrors
// the invariant documented on tacitSpeakerStoppedCallback in speaker_darwin.go:
// that callback must not call stopNative, or speaker_stop's dispatch_sync
// barrier below would deadlock against the queue it is already running on.
static void tacitSpeakerHandleUnexpectedStop(SpeakerCapture* cap, NSString* why) {
    if (atomic_exchange(&cap->notified, true)) return;

    NSLog(@"[tacit] speaker tap stopping: %@", why);
    tacitSpeakerTeardownIO(cap);
    tacitSpeakerStoppedCallback(cap->goHandle);
}

// A global tap follows the system mix, so a change of default output device can
// leave it attached to something that no longer produces audio.  The pipeline's
// stall watchdog cannot catch that: buffers keep arriving, they are just silent
// forever.  Restarting on the change costs one retry cycle and is much cheaper
// than losing capture silently.
static const AudioObjectPropertyAddress kTacitOutputDeviceAddresses[] = {
    { kAudioHardwarePropertyDefaultOutputDevice,
      kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain },
    { kAudioHardwarePropertyDefaultSystemOutputDevice,
      kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain },
};
#define TACIT_OUTPUT_ADDR_COUNT \
    (sizeof(kTacitOutputDeviceAddresses) / sizeof(kTacitOutputDeviceAddresses[0]))

// ---------------------------------------------------------------------------
// speaker_create
// ---------------------------------------------------------------------------

static void tacitSpeakerRelease(SpeakerCapture* cap);

// Renders an OSStatus as both a signed int and, when printable, its FourCC —
// the interesting Core Audio failures here are four-character codes.
static void tacitFormatStatus(OSStatus st, char* out, size_t outLen) {
    union { OSStatus s; unsigned char c[4]; } u;
    u.s = (OSStatus)CFSwapInt32HostToBig((uint32_t)st);
    bool printable = true;
    for (int i = 0; i < 4; i++) {
        if (u.c[i] < 0x20 || u.c[i] > 0x7e) { printable = false; break; }
    }
    if (printable) {
        snprintf(out, outLen, "%d ('%c%c%c%c')", (int)st, u.c[0], u.c[1], u.c[2], u.c[3]);
    } else {
        snprintf(out, outLen, "%d", (int)st);
    }
}

SpeakerCapture* speaker_create(uintptr_t goHandle, char** errMsg) {
    *errMsg = NULL;

    if (@available(macOS 14.2, *)) {
        @autoreleasepool {
            char stbuf[32];

            SpeakerCapture* cap = (SpeakerCapture*)calloc(1, sizeof(SpeakerCapture));
            if (cap == NULL) {
                *errMsg = strdup("out of memory");
                return NULL;
            }
            cap->goHandle    = goHandle;
            cap->tapID       = kAudioObjectUnknown;
            cap->aggregateID = kAudioObjectUnknown;
            atomic_store(&cap->stopped,    false);
            atomic_store(&cap->ioTornDown, false);
            atomic_store(&cap->notified,   false);

            // A global mono mixdown of every process.  Asking the tap for mono
            // means it does the channel downmix, leaving the converter with only
            // a sample-rate change to make.
            CATapDescription* desc =
                [[CATapDescription alloc] initMonoGlobalTapButExcludeProcesses:@[]];
            desc.name         = @"tacit system audio";
            desc.muteBehavior = CATapUnmuted;  // never silence the user's speakers
            [desc setPrivate:YES];

            OSStatus err = AudioHardwareCreateProcessTap(desc, &cap->tapID);
            if (err != noErr || cap->tapID == kAudioObjectUnknown) {
                char buf[192];
                tacitFormatStatus(err, stbuf, sizeof(stbuf));
                snprintf(buf, sizeof(buf),
                         "AudioHardwareCreateProcessTap failed: OSStatus %s — grant audio "
                         "recording permission in System Settings → Privacy & Security", stbuf);
                *errMsg = strdup(buf);
                tacitSpeakerRelease(cap);
                return NULL;
            }

            // Wrap the tap in a private aggregate device that contains nothing
            // else.  Deliberately no sub-devices: adding the real output device
            // would trade the old display dependency for a device dependency,
            // which is exactly the failure this change exists to remove.
            NSString* tapUUID = desc.UUID.UUIDString;
            NSDictionary* aggregateDesc = @{
                @kAudioAggregateDeviceNameKey:          @"tacit system audio",
                @kAudioAggregateDeviceUIDKey:           [@"com.tacit.systemaudio." stringByAppendingString:tapUUID],
                @kAudioAggregateDeviceIsPrivateKey:     @YES,
                @kAudioAggregateDeviceIsStackedKey:     @NO,
                @kAudioAggregateDeviceTapAutoStartKey:  @YES,
                @kAudioAggregateDeviceSubDeviceListKey: @[],
                @kAudioAggregateDeviceTapListKey:       @[ @{
                    @kAudioSubTapUIDKey:               tapUUID,
                    @kAudioSubTapDriftCompensationKey: @YES,
                } ],
            };

            err = AudioHardwareCreateAggregateDevice(
                (__bridge CFDictionaryRef)aggregateDesc, &cap->aggregateID);
            if (err != noErr || cap->aggregateID == kAudioObjectUnknown) {
                char buf[160];
                tacitFormatStatus(err, stbuf, sizeof(stbuf));
                snprintf(buf, sizeof(buf),
                         "AudioHardwareCreateAggregateDevice failed: OSStatus %s", stbuf);
                *errMsg = strdup(buf);
                tacitSpeakerRelease(cap);
                return NULL;
            }

            // Ask the tap what it will actually deliver.  Query only after the
            // aggregate exists — the format reads back empty before that.  Never
            // assume a rate: a wrong one yields speed-shifted audio that Whisper
            // transcribes into confident nonsense, which is worse than failing.
            AudioObjectPropertyAddress fmtAddress = {
                kAudioTapPropertyFormat,
                kAudioObjectPropertyScopeGlobal,
                kAudioObjectPropertyElementMain,
            };
            UInt32 fmtSize = sizeof(cap->tapFormat);
            err = AudioObjectGetPropertyData(cap->tapID, &fmtAddress, 0, NULL,
                                             &fmtSize, &cap->tapFormat);
            if (err != noErr || cap->tapFormat.mSampleRate <= 0.0 ||
                cap->tapFormat.mChannelsPerFrame == 0 || cap->tapFormat.mBytesPerFrame == 0) {
                char buf[160];
                tacitFormatStatus(err, stbuf, sizeof(stbuf));
                snprintf(buf, sizeof(buf),
                         "could not read tap format: OSStatus %s", stbuf);
                *errMsg = strdup(buf);
                tacitSpeakerRelease(cap);
                return NULL;
            }

            // The pipeline wants 16 kHz mono int16 (pkg/audio.SampleRate).
            AudioStreamBasicDescription outFormat;
            memset(&outFormat, 0, sizeof(outFormat));
            outFormat.mSampleRate       = 16000.0;
            outFormat.mFormatID         = kAudioFormatLinearPCM;
            outFormat.mFormatFlags      = kAudioFormatFlagIsSignedInteger | kAudioFormatFlagIsPacked;
            outFormat.mBitsPerChannel   = 16;
            outFormat.mChannelsPerFrame = 1;
            outFormat.mFramesPerPacket  = 1;
            outFormat.mBytesPerFrame    = 2;
            outFormat.mBytesPerPacket   = 2;

            err = AudioConverterNew(&cap->tapFormat, &outFormat, &cap->converter);
            if (err != noErr || cap->converter == NULL) {
                char buf[200];
                tacitFormatStatus(err, stbuf, sizeof(stbuf));
                snprintf(buf, sizeof(buf),
                         "AudioConverterNew failed: OSStatus %s (tap format %.0f Hz / %u ch)",
                         stbuf, cap->tapFormat.mSampleRate,
                         (unsigned)cap->tapFormat.mChannelsPerFrame);
                *errMsg = strdup(buf);
                tacitSpeakerRelease(cap);
                return NULL;
            }

            // 1→1 and 2→1 are handled natively.  Beyond that, take channel 0
            // rather than relying on an implicit downmix.
            if (cap->tapFormat.mChannelsPerFrame > 2) {
                SInt32 channelMap[1] = { 0 };
                AudioConverterSetProperty(cap->converter, kAudioConverterChannelMap,
                                          sizeof(channelMap), channelMap);
            }

            cap->outScratch = (int16_t*)calloc(TACIT_OUT_SCRATCH_FRAMES, sizeof(int16_t));
            if (cap->outScratch == NULL) {
                *errMsg = strdup("out of memory allocating conversion buffer");
                tacitSpeakerRelease(cap);
                return NULL;
            }

            // Listeners go in before the device starts so no change is missed in
            // the startup window.  Delivery is pinned to our own serial queue so
            // speaker_stop can drain it before freeing this struct.
            dispatch_queue_t q = dispatch_queue_create("kr.tacit.speaker.notify",
                                                       DISPATCH_QUEUE_SERIAL);
            cap->notifyQueue = (__bridge_retained void*)q;

            AudioObjectPropertyListenerBlock listener =
                ^(UInt32 inNumberAddresses, const AudioObjectPropertyAddress* inAddresses) {
                    (void)inNumberAddresses;
                    (void)inAddresses;
                    tacitSpeakerHandleUnexpectedStop(cap, @"default output device changed");
                };
            cap->listenerBlock = (__bridge_retained void*)[listener copy];

            cap->listenersInstalled = true;
            for (size_t i = 0; i < TACIT_OUTPUT_ADDR_COUNT; i++) {
                AudioObjectAddPropertyListenerBlock(
                    kAudioObjectSystemObject, &kTacitOutputDeviceAddresses[i], q,
                    (__bridge AudioObjectPropertyListenerBlock)cap->listenerBlock);
            }

            // Install the IOProc only once the converter and scratch exist, so
            // the first cycle can never touch a half-built struct.
            err = AudioDeviceCreateIOProcID(cap->aggregateID, tacitSpeakerIOProc,
                                            cap, &cap->procID);
            if (err != noErr || cap->procID == NULL) {
                char buf[160];
                tacitFormatStatus(err, stbuf, sizeof(stbuf));
                snprintf(buf, sizeof(buf), "AudioDeviceCreateIOProcID failed: OSStatus %s", stbuf);
                *errMsg = strdup(buf);
                tacitSpeakerRelease(cap);
                return NULL;
            }

            err = AudioDeviceStart(cap->aggregateID, cap->procID);
            if (err != noErr) {
                char buf[160];
                tacitFormatStatus(err, stbuf, sizeof(stbuf));
                snprintf(buf, sizeof(buf), "AudioDeviceStart failed: OSStatus %s", stbuf);
                *errMsg = strdup(buf);
                tacitSpeakerRelease(cap);
                return NULL;
            }

            return cap;
        }
    } else {
        *errMsg = strdup("system audio capture requires macOS 14.2 or later "
                         "(Core Audio process taps)");
        return NULL;
    }
}

// ---------------------------------------------------------------------------
// Teardown
// ---------------------------------------------------------------------------

// Tears down whatever has been built so far, in reverse order.  Every step is
// guarded, so this doubles as the error path inside speaker_create.
static void tacitSpeakerRelease(SpeakerCapture* cap) {
    if (cap == NULL) return;

    // Latch first so anything already in flight bails out, and suppress the
    // unexpected-stop notification: this teardown is deliberate.
    atomic_store_explicit(&cap->stopped, true, memory_order_release);
    atomic_store(&cap->notified, true);

    // Remove the listeners, then drain the queue they were delivered on.  Once
    // this dispatch_sync returns, no listener block is running or pending, which
    // is what makes free(cap) below safe.
    if (cap->listenersInstalled && cap->notifyQueue != NULL && cap->listenerBlock != NULL) {
        for (size_t i = 0; i < TACIT_OUTPUT_ADDR_COUNT; i++) {
            AudioObjectRemovePropertyListenerBlock(
                kAudioObjectSystemObject, &kTacitOutputDeviceAddresses[i],
                (__bridge dispatch_queue_t)cap->notifyQueue,
                (__bridge AudioObjectPropertyListenerBlock)cap->listenerBlock);
        }
        cap->listenersInstalled = false;
    }
    if (cap->notifyQueue != NULL) {
        dispatch_sync((__bridge dispatch_queue_t)cap->notifyQueue, ^{});
    }

    // Stop the audio thread before releasing anything it reads.
    tacitSpeakerTeardownIO(cap);

    // Aggregate before tap: the aggregate references the tap by UID, and
    // destroying the tap first leaves the HAL holding a dangling sub-tap.
    if (cap->aggregateID != kAudioObjectUnknown) {
        AudioHardwareDestroyAggregateDevice(cap->aggregateID);
        cap->aggregateID = kAudioObjectUnknown;
    }
    if (cap->tapID != kAudioObjectUnknown) {
        AudioHardwareDestroyProcessTap(cap->tapID);
        cap->tapID = kAudioObjectUnknown;
    }

    // The converter and scratch are read by the IOProc, so they can only go
    // once tacitSpeakerTeardownIO has guaranteed it will not run again.
    if (cap->converter != NULL) {
        AudioConverterDispose(cap->converter);
        cap->converter = NULL;
    }
    if (cap->outScratch != NULL) {
        free(cap->outScratch);
        cap->outScratch = NULL;
    }

    if (cap->listenerBlock != NULL) { CFRelease(cap->listenerBlock); cap->listenerBlock = NULL; }
    if (cap->notifyQueue   != NULL) { CFRelease(cap->notifyQueue);   cap->notifyQueue   = NULL; }

    free(cap);
}

void speaker_stop(SpeakerCapture* cap) {
    tacitSpeakerRelease(cap);
}
