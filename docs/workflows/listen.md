---
source: [tacit/core/workflows/listen]
verified: 4b3ae33
---

# listen

The daemon: runs the [pipeline](../concepts/pipeline.md) from the microphone until stopped, holding the PID file and writing the event log while it runs. `tacit listen` is its only caller.

Owns the cross-cutting behaviours: the order of the stages, restarting a dead source, joining split segments into sessions and the session cap, batching classification, continuing the previous note when the topic carries on, and the never-drop-a-transcript guarantee ([0003](../decisions/0003-never-drop-a-transcript.md)).

Uses every audio component: [mic-recorder](../components/mic-recorder.md), [speech-detector](../components/speech-detector.md), [transcriber](../components/transcriber.md), [model-downloader](../components/model-downloader.md), [note-classifier](../components/note-classifier.md), [note-manager](../components/note-manager.md), [status-reporter](../components/status-reporter.md).

The end-to-end test lives here: it plays a recording through a stand-in microphone ([0010](../decisions/0010-live-audio-only.md)).
