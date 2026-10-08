---
status: accepted
date: 2026-08-26
source: [pkg/pipeline, pkg/process, pkg/storage]
verified: 9825e8e
---

# 0003. Never drop a successfully transcribed segment

**Decision.** Once speech has been transcribed and passed the deterministic gates, it is stored even if classification fails. The only exception is the classifier's deliberate skip, and that is logged with the text.

**Why.** Entries were being lost without a trace: a classifier error, an answer missing fields, a short batch response, or an entry that storage then refused. For a tool whose value is "what you said is not lost", a silent loss is the worst failure. A noisy `unsorted` entry costs the user a glance; a missing one is unrecoverable.

**Consequences.**
- Classification is retried once, then the entry is stored unclassified, with a title taken from its opening words and the `unsorted` category.
- The batch loop walks the items, not the responses, so a short response cannot leave trailing items unvisited.
- All repair happens in one place before storage, so a new refusal rule in storage cannot reopen the hole.
- The same stance applies to the filters: when unsure, keep. See [hallucination-filtering](../concepts/hallucination-filtering.md).
