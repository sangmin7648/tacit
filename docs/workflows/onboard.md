---
source: [tacit/core/workflows/onboard]
verified: f938cbf
---

# onboard

First-run choices (language, speech model, classifier, agent, experimental mode): their defaults, validation, a reachability check for the chosen classifier, applying them to the settings files, installing the skills, and downloading the Whisper model they name. Shared by `tacit setup` and the app's onboarding window so both behave identically. See [configuration](../concepts/configuration.md).

**Recommendations.** It inspects the Mac and returns a recommended answer for every choice with the reason and the facts behind it. The facts come from the components that know them: Ollama, its models and the Claude CLI from [note-classifier](../components/note-classifier.md), memory and the speech-model list from [model-downloader](../components/model-downloader.md), system languages from [setting-manager](../components/setting-manager.md), agents from [skill-installer](../components/skill-installer.md). Front ends show the recommendation beside the user's choice and never apply it unseen. See [0013](../decisions/0013-onboarding-recommends-from-the-environment.md).

**Revision.** A number says which onboarding the user has been through. Whether onboarding is needed is "never set up, or an older revision"; finishing it records the current one. Raise it only when existing users should choose again, never per release.

Refuses invalid choices without writing anything, and backs up a hand-edited legacy reference file before replacing it.
