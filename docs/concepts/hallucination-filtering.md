---
source: [tacit/core/internal/components/note-classifier, tacit/core/internal/components/transcriber, tacit/core/workflows/listen]
verified: a6db5d7
---

# Hallucination filtering

Whisper invents text when fed silence or noise. The output is usually a stock sentence from its training data, often a video outro ("thanks for watching", subtitle credits, news sign-offs). Left alone these fill the knowledge base with fake notes.

Whisper's own non-speech suppression masks only symbol tokens, so hallucinated sentences arrive as ordinary words and must be removed after transcription. Five independent signals do this; none depends on what the speech was about.

| Signal | What it catches | Where |
|---|---|---|
| Filler check | Transcripts made only of hesitation sounds | before classify |
| Phrase denylist | Known stock sentences (built-in plus user additions), matched per sentence ignoring case, spacing, punctuation | after Whisper |
| Repeat run | One sentence repeated back to back, the signature of a decode loop | after Whisper |
| Density gate | Too few characters for the audio length: a stock phrase left over a stretch Whisper read as silence | after Whisper |
| Stock-repeat dedup | The same normalised transcript landing several times inside a rolling window | before classify |

## Design stance: err toward keeping

A false drop destroys real speech silently; a false keep only adds noise the classifier can ignore. So every gate is conservative:

- A sentence is dropped only when a denylisted phrase makes up most of it, so a real sentence containing "thank you" survives.
- Two identical sentences in a row are allowed, since people repeat themselves.
- The dedup window always lets the first occurrences through, so a genuine remark that recurs is not lost.
- The density threshold is set low so only unambiguous cases are caught.

## Why dedup exists

Recurrence is a signal no single-item filter or classifier prompt can see: a hallucination appears dozens of times a day, genuine speech almost never repeats verbatim. See [0005](../decisions/0005-content-agnostic-hallucination-signals.md).

Dropped speech is reported as a `discarded` [event](events.md) with a short reason.
