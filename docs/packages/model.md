---
source: [tacit/pkg/model]
verified: 9825e8e
---

# model

Downloads the configured Whisper model on first use. Writes to a temporary file and renames only when complete, so a cancelled or failed download never leaves a partial model that looks valid. Reports progress through a callback so the CLI can print and the app can draw a bar.
