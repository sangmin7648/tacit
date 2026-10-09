---
source: [tacit/pkg/model]
verified: f0db000
---

# model

Downloads the configured Whisper model, at the end of setup and again on first use if the configured model has changed since. Writes to a temporary file and renames only when complete, so a cancelled or failed download never leaves a partial model that looks valid. Reports progress through a callback so the CLI can print and the app can draw a bar.
