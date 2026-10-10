<script>
  import { onMount } from 'svelte'
  import { settings } from './settings.js'
  import { errorText } from './backend.js'

  // How the window lays settings out. A key missing here — one added to Config
  // later — still shows, under Other, with its key as its label.
  const GROUPS = [
    { title: 'Transcription', keys: ['whisper_model', 'language', 'initial_prompt', 'experimental'] },
    {
      title: 'Classifier',
      keys: ['llm_provider', 'llm_model', 'skill_agent'],
      setUp: true,
      help: 'Changed in Set Up Tacit, which checks the new classifier can be reached before saving it.',
    },
    { title: 'Filtering', advanced: true, keys: ['transcript_denylist', 'dedup_window', 'min_char_rate'] },
    {
      title: 'Speech detection',
      advanced: true,
      keys: ['speech_threshold', 'energy_threshold', 'min_speech_duration', 'silence_duration', 'max_segment_duration', 'max_session_duration'],
    },
  ]

  const LABELS = {
    whisper_model: ['Whisper model', 'The speech-to-text model.'],
    language: ['Language', 'A language code such as en or ko, or auto to detect. Fixing it cuts wrong-language transcripts.'],
    initial_prompt: ['Initial prompt', 'Words to prime transcription with — names, jargon.'],
    experimental: ['Experimental decoding', 'Suppress non-speech tokens and pad speech a little.'],
    llm_provider: ['Provider', ''],
    llm_model: ['Model', ''],
    skill_agent: ['Skill agent', ''],
    transcript_denylist: ['Extra phrases to drop', 'One per line. Sentences made up mostly of one are dropped.'],
    dedup_window: ['Repeat window', 'Drop a transcript stored more than twice within this time. 0s turns it off.'],
    min_char_rate: ['Minimum characters per second', 'Drop transcripts with less text than this for their length. 0 turns it off.'],
    speech_threshold: ['Speech threshold', 'How sure voice detection must be, from 0 to 1.'],
    energy_threshold: ['Energy threshold', 'How loud audio must be to count as speech.'],
    min_speech_duration: ['Shortest speech', 'Shorter speech is ignored.'],
    silence_duration: ['Silence that ends speech', ''],
    max_segment_duration: ['Longest segment', 'Longer speech is split. 0s turns it off.'],
    max_session_duration: ['Longest session', 'Classify after this much continuous speech. 0s turns it off.'],
  }

  let data = $state(null)
  let loadError = $state('')
  let errors = $state({}) // key → message from the last failed change
  let saving = $state({}) // key → true while a change is in flight
  let stale = $state(false) // a change was saved while the daemon was running
  let restarting = $state(false)
  let restartError = $state('')

  const byKey = $derived(Object.fromEntries((data?.fields ?? []).map((f) => [f.key, f])))
  const groups = $derived.by(() => {
    if (!data) return []
    const placed = new Set(GROUPS.flatMap((g) => g.keys))
    const other = data.fields.filter((f) => !placed.has(f.key)).map((f) => f.key)
    return [...GROUPS, ...(other.length ? [{ title: 'Other', keys: other }] : [])].map((g) => ({
      ...g,
      fields: g.keys.map((k) => byKey[k]).filter(Boolean),
    }))
  })

  async function load() {
    try {
      data = await settings.load()
      loadError = ''
    } catch (e) {
      loadError = errorText(e)
    }
  }

  // change runs one Set or Clear and takes the settings it returns. It
  // reports whether the change was saved.
  async function change(key, call) {
    saving[key] = true
    try {
      data = await call()
      delete errors[key]
      if (data.running) stale = true
      return true
    } catch (e) {
      errors[key] = errorText(e)
      return false
    } finally {
      delete saving[key]
    }
  }

  // parse turns what an input holds into the value Set takes, by kind.
  function parse(f, el) {
    switch (f.kind) {
      case 'bool':
        return el.checked
      case 'number': {
        if (el.value.trim() === '') throw new Error('Enter a number')
        return Number(el.value)
      }
      case 'list':
        return el.value.split('\n').map((s) => s.trim()).filter(Boolean)
      default:
        return el.value
    }
  }

  function commit(f, el) {
    let value
    try {
      value = parse(f, el)
    } catch (e) {
      errors[f.key] = e.message
      return
    }
    change(f.key, () => settings.set(f.key, value)).then((saved) => {
      // A refused checkbox would otherwise stay flipped; text inputs keep what
      // was typed, next to the error, to be corrected.
      if (!saved && f.kind === 'bool') el.checked = byKey[f.key].value
    })
  }

  const reset = (f) => change(f.key, () => settings.clear(f.key))

  function shown(v, kind) {
    if (kind === 'list') return v.length ? v.join(', ') : 'none'
    if (v === '') return 'empty'
    return String(v)
  }

  async function restart() {
    restarting = true
    restartError = ''
    try {
      await settings.restart()
      stale = false
      await load()
    } catch (e) {
      restartError = errorText(e)
    } finally {
      restarting = false
    }
  }

  onMount(() => {
    load()
    // The window is hidden, not closed, so pick up edits made elsewhere —
    // `tacit config set`, the file itself — when it comes back.
    window.addEventListener('focus', load)
    return () => window.removeEventListener('focus', load)
  })
</script>

<div class="settings">
  <header class="top">
    <div>
      <h1>Settings</h1>
      <p class="muted lede">Changes save as you make them.</p>
    </div>
  </header>

  {#if stale}
    <div class="banner">
      <span>Tacit is still listening with the old settings.</span>
      <button class="primary" onclick={restart} disabled={restarting}>
        {restarting ? 'Restarting…' : 'Restart Listening'}
      </button>
    </div>
    {#if restartError}<pre class="error">{restartError}</pre>{/if}
  {/if}

  {#if loadError}
    <section>
      <p class="error">Couldn't read the settings:</p>
      <pre class="error">{loadError}</pre>
      <p class="muted">Fix the file with <b>Edit File…</b>; this window reloads when you come back to it.</p>
    </section>
  {/if}

  {#each groups.filter((g) => !g.advanced) as g (g.title)}
    <section>
      <div class="group-head">
        <h2>{g.title}</h2>
        {#if g.setUp}<button onclick={() => settings.setUp()}>Change…</button>{/if}
      </div>
      {#if g.help}<p class="muted help">{g.help}</p>{/if}
      {#each g.fields as f (f.key)}
        {@const [label, help] = LABELS[f.key] ?? [f.key, '']}
        <div class="row" class:overridden={f.overridden}>
          <div class="label">
            <label for={f.key}>{label}</label>
            {#if help}<small>{help}</small>{/if}
            {#if f.overridden}
              <small>
                Default: {shown(f.default, f.kind)} ·
                <button class="link" onclick={() => reset(f)} disabled={saving[f.key]}>Reset</button>
              </small>
            {/if}
          </div>
          <div class="control">
            {#if g.setUp}
              <span id={f.key} class="readonly">{shown(f.value, f.kind)}</span>
            {:else if f.kind === 'bool'}
              <input
                id={f.key}
                type="checkbox"
                checked={f.value}
                disabled={saving[f.key]}
                onchange={(e) => commit(f, e.currentTarget)}
              />
            {:else if f.kind === 'list'}
              <textarea
                id={f.key}
                rows="3"
                spellcheck="false"
                value={f.value.join('\n')}
                disabled={saving[f.key]}
                onchange={(e) => commit(f, e.currentTarget)}
              ></textarea>
            {:else}
              <input
                id={f.key}
                type={f.kind === 'number' ? 'number' : 'text'}
                step="any"
                spellcheck="false"
                value={f.value}
                placeholder={f.kind === 'duration' ? 'e.g. 30s, 5m' : ''}
                disabled={saving[f.key]}
                onchange={(e) => commit(f, e.currentTarget)}
              />
            {/if}
          </div>
          {#if errors[f.key]}<pre class="error field-error">{errors[f.key]}</pre>{/if}
        </div>
      {/each}
    </section>
  {/each}

  <details class="advanced">
    <summary>Advanced</summary>
    <p class="muted help">Tuning for filtering and speech detection. The defaults suit most voices and rooms.</p>
    {#each groups.filter((g) => g.advanced) as g (g.title)}
      <section>
        <div class="group-head">
          <h2>{g.title}</h2>
          {#if g.setUp}<button onclick={() => settings.setUp()}>Change…</button>{/if}
        </div>
        {#if g.help}<p class="muted help">{g.help}</p>{/if}
        {#each g.fields as f (f.key)}
          {@const [label, help] = LABELS[f.key] ?? [f.key, '']}
          <div class="row" class:overridden={f.overridden}>
            <div class="label">
              <label for={f.key}>{label}</label>
              {#if help}<small>{help}</small>{/if}
              {#if f.overridden}
                <small>
                  Default: {shown(f.default, f.kind)} ·
                  <button class="link" onclick={() => reset(f)} disabled={saving[f.key]}>Reset</button>
                </small>
              {/if}
            </div>
            <div class="control">
              {#if g.setUp}
                <span id={f.key} class="readonly">{shown(f.value, f.kind)}</span>
              {:else if f.kind === 'bool'}
                <input
                  id={f.key}
                  type="checkbox"
                  checked={f.value}
                  disabled={saving[f.key]}
                  onchange={(e) => commit(f, e.currentTarget)}
                />
              {:else if f.kind === 'list'}
                <textarea
                  id={f.key}
                  rows="3"
                  spellcheck="false"
                  value={f.value.join('\n')}
                  disabled={saving[f.key]}
                  onchange={(e) => commit(f, e.currentTarget)}
                ></textarea>
              {:else}
                <input
                  id={f.key}
                  type={f.kind === 'number' ? 'number' : 'text'}
                  step="any"
                  spellcheck="false"
                  value={f.value}
                  placeholder={f.kind === 'duration' ? 'e.g. 30s, 5m' : ''}
                  disabled={saving[f.key]}
                  onchange={(e) => commit(f, e.currentTarget)}
                />
              {/if}
            </div>
            {#if errors[f.key]}<pre class="error field-error">{errors[f.key]}</pre>{/if}
          </div>
        {/each}
      </section>
    {/each}
  </details>


  <footer class="file">
    {#if data}<span class="muted path">Saved to <code>{data.path}</code></span>{/if}
    <button onclick={() => settings.editFile()}>Edit File…</button>
  </footer>
</div>

<style>
  .settings { padding: 24px 24px 28px; max-width: 680px; margin: 0 auto; display: grid; gap: 14px; }
  .top { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
  .path { margin: 0; font-size: 12px; -webkit-user-select: text; user-select: text; }
  .banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    border: 1px solid var(--accent);
    border-radius: 10px;
    background: var(--panel);
  }
  section { padding: 12px 18px; }
  .group-head { display: flex; justify-content: space-between; align-items: center; }
  h2 { margin: 4px 0; }
  .help { font-size: 12px; margin: 0 0 4px; }
  .row {
    display: grid;
    grid-template-columns: 1fr minmax(160px, 40%);
    gap: 4px 16px;
    align-items: center;
    padding: 10px 0;
    border-top: 1px solid var(--line);
  }
  .group-head + .row, .help + .row { border-top: none; }
  .label { display: grid; gap: 2px; }
  .label label { font-weight: 600; font-size: 13px; }
  .overridden .label label::after { content: ' •'; color: var(--accent); }
  .control { display: flex; justify-content: flex-end; }
  .control input[type='text'], .control input[type='number'], textarea {
    width: 100%;
    font: inherit;
    padding: 5px 8px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--bg);
    color: var(--text);
  }
  textarea { resize: vertical; }
  input[type='checkbox'] {
    appearance: none;
    width: 34px;
    height: 20px;
    margin: 0;
    border-radius: 999px;
    background: var(--sunken);
    border: 1px solid var(--line);
    position: relative;
    transition: background 0.15s;
  }
  input[type='checkbox']::after {
    content: '';
    position: absolute;
    top: 1px;
    left: 1px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #fff;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    transition: transform 0.15s;
  }
  input[type='checkbox']:checked { background: var(--accent); border-color: var(--accent); }
  input[type='checkbox']:checked::after { transform: translateX(14px); }
  .advanced { display: grid; gap: 14px; }
  .advanced > summary {
    cursor: default;
    font-weight: 600;
    padding: 4px 2px;
    color: var(--muted);
  }
  .advanced[open] > summary { margin-bottom: 8px; }
  .advanced section { margin-bottom: 14px; }
  .file { justify-content: space-between; align-items: center; margin-top: 4px; }
  .file .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .readonly { padding: 5px 0; -webkit-user-select: text; user-select: text; }
  .field-error { grid-column: 1 / -1; margin: 0; }
  button.link {
    padding: 0;
    border: none;
    background: none;
    color: var(--accent);
    font-size: inherit;
  }
</style>
