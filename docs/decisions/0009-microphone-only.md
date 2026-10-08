---
status: accepted
date: 2026-10-09
source: [tacit/pkg/capture, tacit/pkg/config, tacit/cmd/tacit-app]
verified: 9825e8e
---

# 0009. Capture the microphone only

**Decision.** tacit listens to the microphone. Capturing what the Mac plays (system audio) was built and then removed.

**Why.** It cost more than it gave: a second platform-specific capture path in Objective-C, a second macOS permission that forces an app relaunch to take effect and could loop its permission dialog when refused, per-source timing settings, and onboarding steps to manage all of it. A retry cannot fix a refused permission, and an earlier fix for the dialog loop already had to special-case it.

**Consequences.**
- The capture interface stays, so a second source can return, but it is no longer exercised by a real implementation.
- The per-source timing settings collapsed into shared ones; the microphone's old values became the shared defaults, so behaviour is unchanged.
- Stale per-source keys in an override file are ignored, not rejected. See [configuration](../concepts/configuration.md).
- Meetings with remote participants are captured only as far as the microphone hears them.
