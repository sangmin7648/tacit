---
source: [tacit/core/internal/components/setting-manager]
verified: f938cbf
---

# setting-manager

The settings model (defaults, override loading and in-place editing) and the single source of truth for where everything lives under the base directory: settings files, models, PID file, event log. See [configuration](../concepts/configuration.md).

Changing a default or adding a field here means updating the generated reference and the README field tables.

Also reads the system's preferred languages, and keeps which onboarding revision the user has been through in its own small file rather than in the settings, which setup regenerates and the user edits.
