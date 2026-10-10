# Docs log

Append-only. One line per change: date, what, why.

- 2026-10-09: Created the docs wiki: overview, glossary, 11 concept pages, 16 package pages, 9 decisions. Replaced the spec-kit documents removed in 9825e8e. All pages checked against 9825e8e.
- 2026-10-09: Moved the code and Makefile under `tacit/`; `docs/` stays at the repo root. `source` paths now start with `tacit/`.
- 2026-10-09: Removed `tacit process` (decision 0010); events now carry only what the menu bar shows; the Recent menu reads the notes folder; setup downloads the model in the CLI too; daemon status/stop shared. Updated events, pipeline, daemon-and-app, glossary and the audio, pipeline, model, setup, daemon, events, cmd pages; checked against f0db000.
- 2026-10-09: Code reorganised into app/, core/workflows/, core/internal/components/ (decision 0011). Replaced docs/packages/ with docs/apps/, docs/workflows/, docs/components/; re-pointed every `source`; all pages checked against a6db5d7.
- 2026-10-10: Speech that continues the previous note is appended to it instead of starting a new one (decision 0012). Updated classification, knowledge-entry, segmentation, listen and note-classifier pages; checked against 4b3ae33.
- 2026-10-10: Menu-bar usability: Check for Updates answers in a dialog, Recent opens the notes window, the updater verifies and retries reopening the app. Updated desktop, distribution and daemon-and-app; checked against 5396114.
- 2026-10-10: Starting to listen requests the microphone first when the answer is open, so the prompt no longer waits for the model to load. Updated desktop and daemon-and-app; checked against 56b7886.
