# Docs log

Append-only. One line per change: date, what, why.

- 2026-10-09: Created the docs wiki: overview, glossary, 11 concept pages, 16 package pages, 9 decisions. Replaced the spec-kit documents removed in 9825e8e. All pages checked against 9825e8e.
- 2026-10-09: Moved the code and Makefile under `tacit/`; `docs/` stays at the repo root. `source` paths now start with `tacit/`.
- 2026-10-09: Removed `tacit process` (decision 0010); events now carry only what the menu bar shows; the Recent menu reads the notes folder; setup downloads the model in the CLI too; daemon status/stop shared. Updated events, pipeline, daemon-and-app, glossary and the audio, pipeline, model, setup, daemon, events, cmd pages; checked against f0db000.
- 2026-10-09: Code reorganised into app/, core/workflows/, core/internal/components/ (decision 0011). Replaced docs/packages/ with docs/apps/, docs/workflows/, docs/components/; re-pointed every `source`; all pages checked against a6db5d7.
- 2026-10-10: Speech that continues the previous note is appended to it instead of starting a new one (decision 0012). Updated classification, knowledge-entry, segmentation, listen and note-classifier pages; checked against 4b3ae33.
- 2026-10-10: Menu-bar usability: Check for Updates answers in a dialog, Recent opens the notes window, the updater verifies and retries reopening the app. Updated desktop, distribution and daemon-and-app; checked against 5396114.
- 2026-10-10: Starting to listen requests the microphone first when the answer is open, so the prompt no longer waits for the model to load. Updated desktop and daemon-and-app; checked against 56b7886.
- 2026-10-10: Menu-bar glyphs replaced by template icons (not listening, listening, hearing, working, error), the waveform and spinner animated. Updated desktop and daemon-and-app; checked against 56b7886.
- 2026-10-10: Menu-bar errors say what is wrong and offer a fix: the daemon's exit reason is read from its log, and a denied microphone blocks starting. Updated desktop; checked against 46bea8a.
- 2026-10-10: Desktop windows restyled: notes grouped by day with search highlighting and keyboard navigation, settings split into basic and Advanced, onboarding with a pinned footer and a fuller finish screen. Updated desktop; checked against 4ba2a81.
- 2026-10-11: Onboarding inspects the Mac and recommends every choice with its reason (local model always recommended, speech model by memory, language, agent), offers to download the recommended Ollama model, warns about sending transcripts to Anthropic, and carries a revision so existing users are asked once. Added 0013; updated onboard, note-classifier, model-downloader, setting-manager, skill-installer, desktop, cli, configuration and knowledge-entry; checked against f938cbf.
- 2026-10-11: Onboarding offers three speech models (base, small, large-v3-turbo), skips the download step when the model is present, and shows a waiting state while the microphone prompt is open. Updated desktop; checked against 6e2c3cd.
- 2026-10-11: Menu-bar icon drops the working spinner: transcribing and classifying show as listening. Updated desktop; checked against bf2e54a.
