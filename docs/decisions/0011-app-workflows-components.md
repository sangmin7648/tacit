---
status: accepted
date: 2026-10-09
source: [tacit/app, tacit/core, tacit/packaging]
verified: a6db5d7
---

# 0011. Arrange the code as apps, workflows and components

**Decision.** The code is laid out so that its folders say what the system does:

- `app/` holds the binaries (`cli`, `desktop`); `packaging/<os>/` turns them into what a user installs.
- `core/workflows/` holds what an app can ask for, one folder per verb: `listen`, `control`, `onboard`, `browse`, `configure`.
- `core/internal/components/` holds the parts workflows combine, each named for its role with an -er/-or noun: `mic-recorder`, `speech-detector`, `transcriber`, `model-downloader`, `note-classifier`, `note-manager`, `setting-manager`, `skill-installer`, `status-reporter`.

Dependencies run one way: app to workflow to component. Components do not import each other, except `setting-manager` and the audio format in `mic-recorder`; workflows do not import each other. Third-party code and test data live inside the folder that uses them.

**Why.** The previous `pkg/` named packages by mechanism (`process`, `audio`, `events`), so neither a person nor an agent could see from the tree what tacit does or where a change belongs. Go's `internal` rule makes the compiler reject an app that reaches past a workflow, so the layering holds without review; types an app needs are re-exported from workflows as aliases.

**Consequences.**
- Go package names are the folder name without hyphens (`note-manager` is `notemanager`).
- Go links whole packages, so a workflow the app imports must not share a package with audio code. That is why `control` is apart from `listen`, `status-reporter` (PID file, event log) is a component both use, and `model-downloader` is apart from `transcriber`.
- Workflows that only forward to one component (`browse`, `configure`) are the price of the rule; they keep the app's view of core in one list of folders.
- The menu-bar app and the CLI share platform-neutral code; operating-system differences go in file suffixes inside a component, and per-OS bundling in `packaging/`.
