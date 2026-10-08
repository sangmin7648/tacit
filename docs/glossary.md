---
source: [pkg, cmd, skills]
verified: 9825e8e
---

# Glossary

**Knowledge base**: the directory `~/.tacit/` of Markdown entries tacit writes and agents read. Plain files, no database. See [knowledge-entry](concepts/knowledge-entry.md).

**Entry**: one Markdown file holding a title, category, keywords, a summary, and the raw transcript.

**Category**: a single-level directory name under the knowledge base that groups entries (`dev`, `meeting`, ...). Chosen by the classifier; `unsorted` when it could not choose.

**Segment**: a contiguous stretch of speech cut out of the audio stream by VAD, sent to Whisper as one unit.

**Session** (speech session): the run of segments between two long silences. Their text is joined and classified as one item. See [segmentation](concepts/segmentation.md).

**VAD**: voice activity detection. Decides frame by frame whether audio is speech.

**STT**: speech-to-text. Whisper, run locally.

**Transcript**: the text Whisper returns for a segment or session.

**Hallucination**: text Whisper invents over silence or noise, usually a stock sentence from its training data ("thanks for watching"). See [hallucination-filtering](concepts/hallucination-filtering.md).

**Stock repeat**: a transcript that recurs verbatim many times inside the dedup window. The fingerprint of a hallucination.

**Classify**: asking an LLM to give a transcript a title, category, keywords, and summary, or to skip it as meaningless. See [classification](concepts/classification.md).

**Skip**: the classifier's deliberate verdict that a transcript carries nothing worth keeping. The one path that intentionally discards speech.

**Discard**: speech dropped *before* classification by a deterministic gate (filler, hallucination, too sparse, stock repeat).

**Unsorted**: the category given to a transcript the classifier failed on. Stored, never dropped.

**Source**: where audio comes from. Today only the microphone.

**Daemon**: the long-running `tacit listen` process. Whoever starts it (terminal or app), it is the same process with a PID file and an event log.

**Event**: a pipeline moment published as data (speech started, stored, discarded, ...), appended to the event log. See [events](concepts/events.md).

**Override**: the user's own settings file, layered over the generated defaults. See [configuration](concepts/configuration.md).

**Skill**: a Markdown instruction file installed into an AI agent so it can query or add to the knowledge base. See [skills](concepts/skills.md).

**Tacit.app**: the menu-bar Mac app. It bundles the CLI and drives the daemon. See [daemon-and-app](concepts/daemon-and-app.md).
