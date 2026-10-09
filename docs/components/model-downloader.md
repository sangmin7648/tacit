---
source: [tacit/core/internal/components/model-downloader]
verified: a6db5d7
---

# model-downloader

Downloads the configured Whisper model: at the end of [onboard](../workflows/onboard.md), and from [listen](../workflows/listen.md) on first use if the configured model changed since. Writes to a temporary file and renames only when complete, so a cancelled or failed download never leaves a partial model that looks valid. Reports progress through a callback so the CLI can print and the app can draw a bar.

Separate from [transcriber](transcriber.md) because onboarding must not link whisper.
