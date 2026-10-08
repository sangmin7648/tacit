package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation

#import <AVFoundation/AVFoundation.h>

static int micStatus(void) {
	return (int)[AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
}

static void micRequest(void) {
	[AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {}];
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
