package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework CoreGraphics -framework ScreenCaptureKit

#import <AVFoundation/AVFoundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>

static int micStatus(void) {
	return (int)[AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
}

static void micRequest(void) {
	[AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {}];
}

// shareableContentAvailable asks ScreenCaptureKit for shareable content — the
// same first call pkg/capture's speaker source makes before it records system
// audio — and reports whether macOS allowed it. Must not be called on the main
// thread: it waits for a completion handler.
static bool shareableContentAvailable(void) {
	__block bool ok = false;
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	[SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
		ok = (error == nil && content != nil);
		dispatch_semaphore_signal(done);
	}];
	if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC)) != 0) {
		return false;
	}
	return ok;
}

static bool screenPreflight(void) {
	return CGPreflightScreenCaptureAccess();
}

static bool screenRequest(void) {
	return CGRequestScreenCaptureAccess();
}
*/
import "C"

// Permission states as the onboarding window shows them.
const (
	permGranted      = "granted"
	permDenied       = "denied"
	permUndetermined = "undetermined"
	permRestricted   = "restricted"
)

// microphoneStatus reports the app's microphone permission without prompting.
// The bundled daemon runs as the app's child, so this is its permission too.
func microphoneStatus() string {
	switch C.micStatus() {
	case 0: // AVAuthorizationStatusNotDetermined
		return permUndetermined
	case 1: // AVAuthorizationStatusRestricted
		return permRestricted
	case 2: // AVAuthorizationStatusDenied
		return permDenied
	default: // AVAuthorizationStatusAuthorized
		return permGranted
	}
}

// requestMicrophone shows the system prompt if the user has not answered it
// yet; afterwards macOS only changes the answer from System Settings.
func requestMicrophone() { C.micRequest() }

// systemAudioCaptureAllowed reports whether the daemon will be able to record
// system audio: it makes the same ScreenCaptureKit call the daemon does.
//
// While access is missing, macOS answers this call with its "would like to
// record this computer's screen and audio" dialog — every time. Never call it
// from a timer; OnboardingService.VerifyScreenRecording is the only caller and
// rate-limits it.
func systemAudioCaptureAllowed() bool { return bool(C.shareableContentAvailable()) }

// screenRecordingPreflight never prompts, which makes it the one check safe
// to poll. It can under-report: it answers for full screen recording, and has
// stayed false while the daemon was recording system audio under Tacit's
// grant — 22 transcripts in one run. So a false here is not the last word;
// VerifyScreenRecording is.
func screenRecordingPreflight() bool { return bool(C.screenPreflight()) }

// requestScreenRecording prompts once; after that it only reports.
func requestScreenRecording() bool { return bool(C.screenRequest()) }
