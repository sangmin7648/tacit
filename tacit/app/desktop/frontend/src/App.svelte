<script>
  import { onMount } from 'svelte'
  import { backend, errorText } from './backend.js'

  const STEPS = ['Summaries', 'Agent', 'Transcription', 'Speech model', 'Permissions']
  const SPEECH = 3

  let opts = $state(null)
  let rec = $state(null) // what this Mac suggests; null while looking
  let loadError = $state('')
  let choices = $state(null)
  let step = $state(0) // index into STEPS; STEPS.length is the done screen

  // Summaries: the provider must answer before anything is saved, because
  // `tacit listen` refuses to start when it cannot.
  let checking = $state(false)
  let checkError = $state('')
  let checkedOK = $state(false)
  let rescanning = $state(false)

  // Pulling the recommended Ollama model
  let pull = $state(null) // the in-flight (cancellable) call, or null
  let pullProgress = $state({ done: 0, total: 0 })
  let pullError = $state('')

  // Transcription
  let saving = $state(false)
  let saveError = $state('')

  // Speech model
  let download = $state(null)
  let progress = $state({ done: 0, total: 0 })
  let modelError = $state('')

  // Permissions
  let perms = $state(null)

  const percent = $derived(progress.total > 0 ? Math.floor((progress.done / progress.total) * 100) : 0)
  const pullPercent = $derived(pullProgress.total > 0 ? Math.floor((pullProgress.done / pullProgress.total) * 100) : 0)

  const installedOllama = $derived(rec?.ollama.models ?? [])
  const ollamaHas = (name) => installedOllama.some((m) => m === name || m === name + ':latest')
  const modelInstalled = $derived(ollamaHas(choices?.llm_model))
  const ollamaSuggestions = $derived(
    [...new Set([opts?.default_ollama_model, ...installedOllama.map((m) => m.replace(/:latest$/, ''))])].filter(Boolean),
  )

  // The speech models offered, plus the one already configured if it is not
  // among them (set by hand in the settings file).
  const whisperModels = $derived.by(() => {
    const list = rec?.whisper_models ?? []
    const cur = choices?.whisper_model
    if (cur && !list.some((m) => m.name === cur)) {
      return [...list, { name: cur, custom: true, installed: opts?.model.present && opts.model.name === cur }]
    }
    return list
  })

  onMount(() => {
    // Detection talks to Ollama, so it can take a moment: show the form's
    // shape at once and fill in the recommendations when they arrive.
    Promise.all([backend.options(), backend.recommend()])
      .then(([o, r]) => {
        opts = o
        rec = r
        // First run starts from the recommendation. Afterwards it starts from
        // what the user has; the recommendation is shown beside it.
        choices = { ...(o.configured ? o.choices : r.choices) }
      })
      .catch((e) => (loadError = errorText(e)))
    const offModel = backend.onModelProgress((p) => (progress = p))
    const offPull = backend.onPullProgress((p) => (pullProgress = p))
    // Permissions change in System Settings, outside this window: poll while
    // that step is showing.
    const poll = setInterval(refreshPerms, 1000)
    return () => {
      offModel()
      offPull()
      clearInterval(poll)
    }
  })

  function setProvider(p) {
    choices.llm_provider = p
    if (p === 'claude' && !opts.claude_models.includes(choices.llm_model)) {
      choices.llm_model = opts.claude_models[0]
    } else if (p === 'ollama' && opts.claude_models.includes(choices.llm_model)) {
      choices.llm_model = opts.default_ollama_model
    }
    resetCheck()
  }

  function resetCheck() {
    checkedOK = false
    checkError = ''
  }

  function useRecommended(...keys) {
    for (const k of keys) choices[k] = rec.choices[k]
    resetCheck()
  }

  const differs = (...keys) => keys.some((k) => choices[k] !== rec.choices[k])

  async function rescan() {
    rescanning = true
    try {
      rec = await backend.recommend()
    } finally {
      rescanning = false
    }
  }

  async function check() {
    checking = true
    resetCheck()
    try {
      await backend.checkProvider($state.snapshot(choices))
      checkedOK = true
    } catch (e) {
      checkError = errorText(e)
    } finally {
      checking = false
    }
  }

  async function startPull() {
    pullError = ''
    pullProgress = { done: 0, total: 0 }
    pull = backend.pullOllamaModel(choices.llm_model)
    try {
      await pull
      await rescan()
      await check()
    } catch (e) {
      pullError = errorText(e)
    } finally {
      pull = null
    }
  }

  async function save() {
    saving = true
    saveError = ''
    try {
      await backend.apply($state.snapshot(choices))
      opts = await backend.options() // the speech model step needs the new model
      step = SPEECH
    } catch (e) {
      saveError = errorText(e)
    } finally {
      saving = false
    }
  }

  async function startDownload() {
    modelError = ''
    progress = { done: 0, total: 0 }
    download = backend.downloadModel()
    try {
      await download
      opts.model.present = true
    } catch (e) {
      modelError = errorText(e)
    } finally {
      download = null
    }
  }

  async function refreshPerms() {
    if (step === SPEECH + 1) perms = await backend.permissions()
  }

  async function goToPermissions() {
    step = SPEECH + 1
    await refreshPerms()
  }

  const gb = (n) => (n / 1e9).toFixed(2)
  const size = (mb) => (mb >= 1000 ? (mb / 1000).toFixed(1) + ' GB' : mb + ' MB')
</script>

<main>
  <header>
    <h1>Set up Tacit</h1>
    <p class="lede">
      {#if opts?.configured}
        Tacit now looks at your Mac and recommends settings. Your current choices are kept unless you change them.
      {:else}
        Tacit listens on this Mac, transcribes on-device, and files what it hears as notes. We looked at your Mac and
        filled in what we recommend.
      {/if}
    </p>
    <ol class="steps">
      {#each STEPS as name, i}
        <li class:current={i === step} class:done={i < step}>{name}</li>
      {/each}
    </ol>
  </header>

  {#snippet advice(keys, field)}
    <p class="advice">
      {#if !differs(...keys)}
        <span class="tag">Recommended</span>
      {/if}
      {rec.reasons[field]}
      {#if differs(...keys)}
        <button class="link" onclick={() => useRecommended(...keys)}>Use the recommendation</button>
      {/if}
    </p>
  {/snippet}

  {#if loadError}
    <p class="error">Couldn't read your configuration: {loadError}</p>
  {:else if !choices}
    <p class="muted">Looking at your Mac…</p>
  {:else if step === 0}
    <section>
      <h2>Who writes the summaries?</h2>
      <p class="muted">Each transcript is titled, filed and summarised by a language model.</p>

      <ul class="findings">
        <li class:found={rec.ollama.running}>
          <span aria-hidden="true">{rec.ollama.running ? '✓' : '–'}</span>
          {#if rec.ollama.running}Ollama is running ({rec.ollama.models.length} model{rec.ollama.models.length === 1 ? '' : 's'})
          {:else if rec.ollama.installed}Ollama is installed but not running
          {:else}Ollama was not found{/if}
        </li>
        <li class:found={rec.claude_available}>
          <span aria-hidden="true">{rec.claude_available ? '✓' : '–'}</span>
          {rec.claude_available ? 'Claude Code CLI found' : 'Claude Code CLI was not found'}
        </li>
        <li><button class="link" onclick={rescan} disabled={rescanning}>{rescanning ? 'Looking…' : 'Look again'}</button></li>
      </ul>

      <div class="choices" role="radiogroup" aria-label="Summary provider">
        <label class="option" class:selected={choices.llm_provider === 'ollama'}>
          <input type="radio" name="provider" checked={choices.llm_provider === 'ollama'} onchange={() => setProvider('ollama')} />
          <span>
            <strong>Ollama {#if rec.choices.llm_provider === 'ollama'}<span class="tag">Recommended</span>{/if}</strong>
            <small>A model running locally. Nothing leaves this Mac.</small>
          </span>
        </label>
        <label class="option" class:selected={choices.llm_provider === 'claude'}>
          <input type="radio" name="provider" checked={choices.llm_provider === 'claude'} onchange={() => setProvider('claude')} />
          <span>
            <strong>Claude {#if rec.choices.llm_provider === 'claude'}<span class="tag">Recommended</span>{/if}</strong>
            <small>Through the Claude Code CLI. The text of what you say is sent to Anthropic.</small>
          </span>
        </label>
      </div>

      {#if choices.llm_provider === 'claude'}
        <p class="callout">
          <strong>Your transcripts leave this Mac.</strong> Tacit still transcribes on-device and never sends audio,
          but each transcript's text goes to Anthropic's servers to be titled and filed. Choose Ollama to keep
          everything local.
        </p>
      {/if}

      {#if rec.memory_warning && choices.llm_provider === 'ollama'}
        <p class="callout warn"><strong>Memory is tight.</strong> {rec.memory_warning}</p>
      {/if}

      {@render advice(['llm_provider', 'llm_model'], 'llm_provider')}

      <label class="field">
        <span>Model</span>
        {#if choices.llm_provider === 'claude'}
          <select bind:value={choices.llm_model} onchange={resetCheck}>
            {#each opts.claude_models as m}<option value={m}>{m}</option>{/each}
          </select>
        {:else}
          <input type="text" list="ollama-models" bind:value={choices.llm_model} oninput={resetCheck}
            placeholder={opts.default_ollama_model} spellcheck="false" />
          <datalist id="ollama-models">
            {#each ollamaSuggestions as m}<option value={m}></option>{/each}
          </datalist>
        {/if}
      </label>
      {#if choices.llm_provider === rec.choices.llm_provider}
        <p class="advice">
          {#if choices.llm_model === rec.choices.llm_model}<span class="tag">Recommended</span>{/if}
          {rec.reasons.llm_model}
        </p>
      {/if}

      {#if choices.llm_provider === 'ollama' && !modelInstalled && choices.llm_model.trim()}
        {#if pull}
          <progress max="100" value={pullPercent}></progress>
          <p class="muted">
            Downloading {choices.llm_model}…
            {#if pullProgress.total > 0}{gb(pullProgress.done)} of {gb(pullProgress.total)} GB ({pullPercent}%){/if}
          </p>
        {:else if rec.ollama.running}
          <p class="advice">
            <code>{choices.llm_model}</code> is not installed in Ollama.
            <button onclick={startPull}>Download {choices.llm_model}</button>
          </p>
        {/if}
        {#if pullError}<pre class="error">{pullError}</pre>{/if}
      {/if}

      <div class="status">
        {#if checking}
          <span class="muted">Checking…</span>
        {:else if checkedOK}
          <span class="ok">✓ Ready</span>
        {:else if checkError}
          <pre class="error">{checkError}</pre>
        {/if}
      </div>

      <footer>
        {#if pull}
          <button onclick={() => pull.cancel()}>Cancel download</button>
        {:else}
          <button onclick={check} disabled={checking || !choices.llm_model.trim()}>Check connection</button>
          <button class="primary" onclick={() => (step = 1)} disabled={!checkedOK}>Next</button>
        {/if}
      </footer>
    </section>
  {:else if step === 1}
    <section>
      <h2>Which AI agent should find your notes?</h2>
      <p class="muted">Tacit installs a skill so the agent can search what you've said.</p>

      <div class="choices" role="radiogroup" aria-label="Agent">
        {#each rec.agents as a}
          <label class="option" class:selected={choices.skill_agent === a.name}>
            <input type="radio" name="agent" checked={choices.skill_agent === a.name} onchange={() => (choices.skill_agent = a.name)} />
            <span>
              <strong>{a.label} {#if a.recommended}<span class="tag">Recommended</span>{/if}</strong>
              <small>{a.installed ? 'Found on this Mac.' : 'Not found on this Mac.'}</small>
            </span>
          </label>
        {/each}
      </div>
      {@render advice(['skill_agent'], 'skill_agent')}

      <footer>
        <button onclick={() => (step = 0)}>Back</button>
        <button class="primary" onclick={() => (step = 2)}>Next</button>
      </footer>
    </section>
  {:else if step === 2}
    <section>
      <h2>How should it transcribe?</h2>
      <p class="muted">Tacit listens to your microphone.</p>

      <label class="field">
        <span>Language</span>
        <select bind:value={choices.language}>
          {#each opts.languages as l}
            <option value={l.code}>{l.label}{l.code === rec.choices.language ? ' (recommended)' : ''}</option>
          {/each}
        </select>
      </label>
      {@render advice(['language'], 'language')}

      <div class="field">
        <span>Speech model</span>
        <div class="choices" role="radiogroup" aria-label="Speech model">
          {#each whisperModels as m}
            <label class="option" class:selected={choices.whisper_model === m.name}>
              <input type="radio" name="whisper" checked={choices.whisper_model === m.name} onchange={() => (choices.whisper_model = m.name)} />
              <span>
                <strong>{m.name} {#if m.recommended}<span class="tag">Recommended</span>{/if}</strong>
                <small>
                  {#if m.custom}Set in your settings file.
                  {:else}Uses about {size(m.ram_mb)} of RAM · {size(m.download_mb)} download{/if}
                  {#if m.installed} · on this Mac{/if}
                </small>
              </span>
            </label>
          {/each}
        </div>
      </div>
      {@render advice(['whisper_model'], 'whisper_model')}

      <label class="check">
        <input type="checkbox" bind:checked={choices.experimental} />
        <span><strong>Experimental transcription</strong><small>Suppresses non-speech tokens and pads speech onsets.</small></span>
      </label>

      {#if saveError}<pre class="error">{saveError}</pre>{/if}
      <footer>
        <button onclick={() => (step = 1)}>Back</button>
        <button class="primary" onclick={save} disabled={saving}>{saving ? 'Saving…' : 'Save and continue'}</button>
      </footer>
    </section>
  {:else if step === SPEECH}
    <section>
      <h2>Speech model</h2>
      {#if opts.model.present}
        <p class="ok">✓ <code>{opts.model.name}</code> is on this Mac.</p>
      {:else}
        <p class="muted">
          Transcription runs on <code>{opts.model.name}</code>, which has to be downloaded once. A fast connection helps.
        </p>
        {#if download}
          <progress max="100" value={percent}></progress>
          <p class="muted">
            {#if progress.total > 0}{gb(progress.done)} of {gb(progress.total)} GB ({percent}%){:else}Starting…{/if}
          </p>
        {/if}
        {#if modelError}<pre class="error">{modelError}</pre>{/if}
      {/if}
      <footer>
        {#if download}
          <button onclick={() => download.cancel()}>Cancel</button>
        {:else}
          <button onclick={() => (step = 2)}>Back</button>
          {#if opts.model.present}
            <button class="primary" onclick={goToPermissions}>Next</button>
          {:else}
            <button onclick={goToPermissions}>Skip for now</button>
            <button class="primary" onclick={startDownload}>Download</button>
          {/if}
        {/if}
      </footer>
    </section>
  {:else if step === SPEECH + 1}
    <section>
      <h2>Permissions</h2>
      <p class="muted">macOS asks once. You can change it later in System Settings → Privacy &amp; Security.</p>
      {#if !perms}
        <p class="muted">Checking…</p>
      {:else}
        <div class="perm">
          <span><strong>Microphone</strong></span>
          {#if perms.microphone === 'granted'}
            <span class="pill ok">✓ Allowed</span>
          {:else if perms.microphone === 'undetermined'}
            <button onclick={() => backend.requestMicrophone()}>Allow…</button>
          {:else}
            <button onclick={() => backend.openPrivacySettings('Microphone')}>Open Settings</button>
          {/if}
        </div>
      {/if}
      <footer>
        <button onclick={() => (step = SPEECH)}>Back</button>
        <button class="primary" onclick={() => (step = SPEECH + 2)}>Next</button>
      </footer>
    </section>
  {:else}
    <section class="done">
      <div class="badge" aria-hidden="true">✓</div>
      <h2>You're set up</h2>
      <ul class="tips">
        <li><strong>Menu bar</strong><small>Tacit lives there. The icon shows when it is listening, hearing you, or working.</small></li>
        <li><strong>Notes</strong><small>Everything you say lands in <code>~/.tacit</code> as Markdown. Open Notes from the menu to search it.</small></li>
        <li><strong>In Claude</strong><small><code>/tacit.knowledge</code> finds your notes from inside Claude.</small></li>
      </ul>
      {#if !opts.model.present}
        <p class="muted">The speech model isn't downloaded yet; Tacit will fetch it the first time it starts listening.</p>
      {/if}
      <footer>
        <button onclick={() => backend.finish(false)}>Close</button>
        <button class="primary" onclick={() => backend.finish(true)}>Start listening</button>
      </footer>
    </section>
  {/if}
</main>

<style>
  .done { align-items: flex-start; }
  .badge {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 22px;
    font-weight: 700;
    background: var(--ok);
    color: var(--panel);
    margin-bottom: 12px;
  }
  .tips { list-style: none; margin: 8px 0 0; padding: 0; display: grid; gap: 14px; }
  .tips li { display: grid; gap: 1px; }
  .tag {
    display: inline-block;
    margin-right: 4px;
    padding: 0 7px;
    border-radius: 999px;
    background: var(--accent-soft);
    color: var(--accent);
    font-size: 11px;
    font-weight: 600;
    vertical-align: 1px;
  }
  .advice { margin: 6px 0 0; color: var(--muted); font-size: 12px; }
  .callout {
    margin: 8px 0 0;
    padding: 10px 12px;
    border-radius: 9px;
    border: 1px solid var(--error);
    font-size: 12px;
  }
  .callout.warn { border-color: var(--line); background: var(--sunken); }
  .findings { list-style: none; margin: 8px 0 4px; padding: 0; display: grid; gap: 2px; color: var(--muted); }
  .findings li.found { color: var(--text); }
  .findings li span { display: inline-block; width: 14px; }
  button.link {
    border: none;
    box-shadow: none;
    background: none;
    padding: 0 2px;
    color: var(--accent);
    text-decoration: underline;
  }
  .pill { padding: 2px 10px; border-radius: 999px; background: var(--accent-soft); font-size: 12px; }
</style>
