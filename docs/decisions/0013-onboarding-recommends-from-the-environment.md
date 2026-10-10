---
status: accepted
date: 2026-10-11
---

# Onboarding recommends from the environment

## Context

Onboarding started every user from the same defaults: a local Ollama model, language auto, the largest speech model. Whoever had no Ollama, or a small Mac, or spoke one language, met the consequence later, as a daemon that would not start, a slow transcription or wrong-language text, and the choices had been made without them knowing what was available.

## Decision

Onboarding inspects the Mac (Ollama and its models, the Claude Code CLI, memory, the system languages, the agents installed, the speech models already downloaded) and recommends an answer to every question. Each recommendation is shown with its reason beside the user's own choice, including when the two differ, and nothing is chosen silently.

- The local model is always the recommended classifier, even when Ollama is not installed yet, because keeping speech on the Mac is the point of tacit. A Mac with too little memory for it is told so instead of being steered to Claude, and choosing Claude warns that transcript text, never audio, is sent to Anthropic.
- A recommended Ollama model that is not installed can be downloaded from the window.
- The speech model follows memory: large-v3-turbo from 16 GB, because it needs about what medium does and transcribes better; smaller below.

Onboarding carries a revision number, recorded when the user finishes it. The app opens it on a first run and once for each user whose recorded revision is older than the current one; users already set up are marked as soon as it is shown. Bumping the number is how a later release brings existing users back, and ordinary updates do not.

## Why

- A recommendation the user cannot see or override is a default in disguise; showing the reason lets them disagree.
- Tying the window to a revision rather than the app version keeps updates quiet and still lets a release that changes the choices reach people who are already set up.
- Detection belongs to the components that already know each subject, and the workflow combines it, so the window and `tacit setup` give the same advice.

## Consequences

- Recommendations are rules, not measurements: a Mac's real speed is not tested.
- A change to the options or to what is recommended needs the revision raised to be seen by existing users.
