---
source: [tacit/pkg/events, tacit/pkg/pipeline]
verified: 9825e8e
---

# Events

The pipeline narrates itself twice: as log lines for humans, and as **events** for programs. An event is one observable moment: listening, speech started or ended, transcribing, transcribed, discarded (with a reason), classifying, stored, skipped, or an absorbed error.

## Why events exist

A second front end (the menu-bar app) would otherwise have had to scrape English log prose to learn that an entry was stored. Events are that narration as data. Emitting one never replaces the log line beside it.

## Transport

The daemon appends events to a line-delimited log file regardless of who started it. A file, not the child's stdout, because the app must be able to watch a daemon it did not spawn, such as one started from a terminal. One older generation is kept when the file rotates, a reader that arrives mid-write skips a torn line, and a follower survives rotation.

## Rules

- **Additive kinds.** A reader ignores kinds it does not know, so a newer daemon stays readable by an older app.
- **Missing means unknown.** Most fields apply to only some kinds; absence is not an empty value.
- **Logging cannot hurt the daemon.** A failure to write the event log is recorded but never stops the pipeline.
- **Observers are fast.** They run inline with audio handling and must buffer anything slow.
