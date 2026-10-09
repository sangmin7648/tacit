---
source: [tacit/packaging/mac, tacit/Makefile]
verified: a6db5d7
---

# packaging/mac: Tacit.app

Lays the two binaries out as Tacit.app with its Info.plist and the VAD framework, stamps the version, and signs the bundle. `make app` builds the binaries and calls it.

The CLI sits in `Contents/Helpers`, not beside the app: the default macOS filesystem is case-insensitive, so `tacit` would overwrite `Tacit`. Ad-hoc signing ties the microphone grant to one build's hash, so each rebuild asks again; a stable identity avoids that. See [distribution](../concepts/distribution.md).
