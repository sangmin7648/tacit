---
source: [tacit/pkg/events, tacit/pkg/pipeline]
verified: f0db000
---

# Events

An **event** tells a front end what the daemon is doing right now: listening, speech started or ended, transcribing, transcribed, discarded, classifying, stored, or skipped. It carries only its kind, its time, and the capture source.

## Why events exist

The daemon is its own process, so the menu-bar app has no other way to know whether it is hearing speech or working on it. A stored event also tells the app to reload its notes.

## What events do not carry

What was said, why a segment was dropped, and what went wrong are in the daemon log, the record a person reads to diagnose a run. Events once duplicated them; no reader used the copies, and they spread transcripts into a second file.

## Transport

The daemon appends events to a line-delimited log file regardless of who started it. A file, not the child's stdout, because the app must be able to watch a daemon it did not spawn, such as one started from a terminal. One older generation is kept when the file rotates, a reader that arrives mid-write skips a torn line, and a follower survives rotation.

## Rules

- **Additive kinds.** A reader ignores kinds it does not know, so a newer daemon stays readable by an older app.
- **Logging cannot hurt the daemon.** A failure to write the event log is recorded but never stops the pipeline.
- **Observers are fast.** They run inline with audio handling and must buffer anything slow.
