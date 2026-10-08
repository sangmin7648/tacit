---
source: [pkg/setup]
verified: 9825e8e
---

# setup

First-run choices (language, classifier, agent, experimental mode): their defaults, validation, a reachability check for the chosen classifier, and applying them to the settings files. Shared by `tacit setup` and the app's onboarding window so both behave identically. See [configuration](../concepts/configuration.md).

Refuses invalid choices without writing anything, and backs up a hand-edited legacy reference file before replacing it.
