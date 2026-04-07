// speaker_darwin.m — ScreenCaptureKit system-audio capture for macOS 13+.
// Compiled as Objective-C by CGo on Darwin.

#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreMedia/CoreMedia.h>
#import <AudioToolbox/AudioToolbox.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>

#include "speaker_darwin.h"

// ---------------------------------------------------------------------------
// Stream output delegate
// ---------------------------------------------------------------------------

API_AVAILABLE(macos(13.0))
@interface TacitSpeakerOutput : NSObject <SCStreamOutput, SCStreamDelegate>
@property (atomic) uintptr_t goHandle;
@property (atomic) BOOL stopped;
@end

@implementation TacitSpeakerOutput

- (void)stream:(SCStream *)stream
    didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer
    ofType:(SCStreamOutputType)type API_AVAILABLE(macos(12.3)) {

    @autoreleasepool {
        if (type != SCStreamOutputTypeAudio) return;
        if (self.stopped) return;

        AudioBufferList audioBufferList;
        CMBlockBufferRef blockBuffer = NULL;

        OSStatus status = CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(
            sampleBuffer,
            NULL,
            &audioBufferList,
            sizeof(audioBufferList),
            NULL,
            NULL,
            kCMSampleBufferFlag_AudioBufferList_Assure16ByteAlignment,
            &blockBuffer
        );

        if (status != noErr || blockBuffer == NULL) return;

        if (audioBufferList.mNumberBuffers < 1) {
            CFRelease(blockBuffer);
            return;
        }

        // Mono: one buffer, float32 samples
        AudioBuffer* buf = &audioBufferList.mBuffers[0];
        float* floatSamples = (float*)buf->mData;
        int frameCount = (int)(buf->mDataByteSize / sizeof(float));

        if (frameCount <= 0) {
            CFRelease(blockBuffer);
            return;
        }

        // Convert float32 [-1, 1] → int16
        int16_t* intSamples = (int16_t*)malloc((size_t)frameCount * sizeof(int16_t));
        if (!intSamples) {
            CFRelease(blockBuffer);
            return;
        }

        for (int i = 0; i < frameCount; i++) {
            float f = floatSamples[i];
            if (f > 1.0f)  f = 1.0f;
            if (f < -1.0f) f = -1.0f;
            intSamples[i] = (int16_t)(f * 32767.0f);
        }

        tacitSpeakerSamplesCallback(self.goHandle, intSamples, frameCount);

        free(intSamples);
        CFRelease(blockBuffer);
    }
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error API_AVAILABLE(macos(12.3)) {
    if (!self.stopped) {
        if (error) {
            NSLog(@"[tacit] speaker stream stopped with error: %@", error.localizedDescription);
        } else {
            NSLog(@"[tacit] speaker stream stopped unexpectedly (no error)");
        }
        // Notify Go so it can close the channel and let the pipeline restart.
        tacitSpeakerStoppedCallback(self.goHandle);
    }
}

@end

// ---------------------------------------------------------------------------
// SpeakerCapture struct
// ---------------------------------------------------------------------------

struct SpeakerCapture {
    // Use void* to avoid ObjC types in the C header.
    void* stream;  // SCStream* (CFRetained)
    void* output;  // TacitSpeakerOutput* (CFRetained)
};

// ---------------------------------------------------------------------------
// speaker_create
// ---------------------------------------------------------------------------

SpeakerCapture* speaker_create(uintptr_t goHandle, char** errMsg) {
    *errMsg = NULL;

    if (@available(macOS 13.0, *)) {
        // Use __block so we can assign inside the completion handler.
        __block SCShareableContent* content = nil;
        __block NSError* contentError = nil;

        dispatch_semaphore_t sem = dispatch_semaphore_create(0);

        [SCShareableContent getShareableContentWithCompletionHandler:^(
            SCShareableContent* c, NSError* e) {
            content = c;
            contentError = e;
            dispatch_semaphore_signal(sem);
        }];

        // Wait up to 10 seconds for permission / content enumeration.
        long result = dispatch_semaphore_wait(
            sem, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));

        if (result != 0) {
            *errMsg = strdup("timed out waiting for SCShareableContent (Screen Recording permission may be missing)");
            return NULL;
        }

        if (contentError != nil) {
            const char* desc = [[contentError localizedDescription] UTF8String];
            *errMsg = strdup(desc ? desc : "SCShareableContent error");
            return NULL;
        }

        if (content == nil || content.displays.count == 0) {
            *errMsg = strdup("no displays found — cannot create audio filter");
            return NULL;
        }

        // Use the first (main) display; exclude no windows → captures all app audio.
        SCDisplay* display = content.displays[0];
        SCContentFilter* filter = [[SCContentFilter alloc]
            initWithDisplay:display
            excludingWindows:@[]];

        SCStreamConfiguration* config = [[SCStreamConfiguration alloc] init];
        config.capturesAudio        = YES;
        config.sampleRate           = 16000;
        config.channelCount         = 1;
        // Minimise video overhead — we don't use it but SCStream still
        // requires a non-zero size. Use the smallest allowed value.
        config.width                = 2;
        config.height               = 2;
        config.minimumFrameInterval = CMTimeMake(1, 1); // 1 fps — negligible overhead

        TacitSpeakerOutput* output = [[TacitSpeakerOutput alloc] init];
        output.goHandle = goHandle;
        output.stopped  = NO;

        NSError* streamError = nil;
        SCStream* stream = [[SCStream alloc] initWithFilter:filter
                                              configuration:config
                                                   delegate:output];

        BOOL added = [stream addStreamOutput:output
                                        type:SCStreamOutputTypeAudio
                         sampleHandlerQueue:dispatch_get_global_queue(
                             DISPATCH_QUEUE_PRIORITY_HIGH, 0)
                                       error:&streamError];
        if (!added || streamError != nil) {
            const char* desc = [[streamError localizedDescription] UTF8String];
            *errMsg = strdup(desc ? desc : "failed to add stream output");
            return NULL;
        }

        // Start the stream (async completion).
        __block NSError* startError = nil;
        dispatch_semaphore_t startSem = dispatch_semaphore_create(0);

        [stream startCaptureWithCompletionHandler:^(NSError* e) {
            startError = e;
            dispatch_semaphore_signal(startSem);
        }];

        dispatch_semaphore_wait(
            startSem, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));

        if (startError != nil) {
            const char* desc = [[startError localizedDescription] UTF8String];
            *errMsg = strdup(desc ? desc : "failed to start SCStream");
            return NULL;
        }

        // Retain Obj-C objects across the CGo boundary using CFBridgingRetain.
        SpeakerCapture* cap = (SpeakerCapture*)calloc(1, sizeof(SpeakerCapture));
        cap->stream = (void*)CFBridgingRetain(stream);
        cap->output = (void*)CFBridgingRetain(output);

        return cap;
    } else {
        *errMsg = strdup("system audio capture requires macOS 13.0 or later");
        return NULL;
    }
}

// ---------------------------------------------------------------------------
// speaker_stop
// ---------------------------------------------------------------------------

void speaker_stop(SpeakerCapture* cap) {
    if (!cap) return;

    if (@available(macOS 13.0, *)) {
        if (cap->output) {
            TacitSpeakerOutput* output =
                (__bridge TacitSpeakerOutput*)cap->output;
            output.stopped = YES;
        }

        if (cap->stream) {
            SCStream* stream = (__bridge SCStream*)cap->stream;
            [stream stopCaptureWithCompletionHandler:^(NSError* e) {
                // Ignore stop errors.
            }];
            CFRelease(cap->stream);
            cap->stream = NULL;
        }

        if (cap->output) {
            CFRelease(cap->output);
            cap->output = NULL;
        }
    }

    free(cap);
}
