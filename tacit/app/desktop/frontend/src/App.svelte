<script>
  import { onMount } from 'svelte'
  import { backend, errorText } from './backend.js'

  const STEPS = ['Summaries', 'Transcription', 'Speech model', 'Permissions']

  let opts = $state(null)
  let loadError = $state('')
  let choices = $state(null)
  let step = $state(0) // index into STEPS; STEPS.length is the done screen

  // Summaries: the provider must answer before anything is saved, because
  // `tacit listen` refuses to start when it cannot.
  let checking = $state(false)
  let checkError = $state('')
  let checkedOK = $state(false)

  // Transcription
  let saving = $state(false)
  let saveError = $state('')

  // Speech model
  let download = $state(null) // the in-flight (cancellable) call, or null
  let progress = $state({ done: 0, total: 0 })
  let modelError = $state('')

  // Permissions
  let perms = $state(null)

  const percent = $derived(progress.total > 0 ? Math.floor((progress.done / progress.total) * 100) : 0)

  onMount(() => {
    backend
      .options()
      .then((o) => {
        opts = o
        choices = { ...o.choices }
      })
      .catch((e) => (loadError = errorText(e)))
    const off = backend.onModelProgress((p) => (progress = p))
    // Permissions change in System Settings, outside this window: poll while
    // that step is showing.
    const poll = setInterval(refreshPerms, 1000)
    return () => {
      off()
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

  async function save() {
    saving = true
    saveError = ''
    try {
      await backend.apply($state.snapshot(choices))
      step = 2
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

  function cancelDownload() {
    download?.cancel()
  }

  async function refreshPerms() {
    if (step === 3) perms = await backend.permissions()
  }

  async function goToPermissions() {
    step = 3
    await refreshPerms()
  }

  function gb(n) {
    return (n / 1e9).toFixed(2)
  }
</script>

<main>
  <header>
    <h1>Set up Tacit</h1>
    <p class="lede">Tacit listens on this Mac, transcribes on-device, and files what it hears as notes.</p>
    <ol class="steps">
      {#each STEPS as name, i}
        <li class:current={i === step} class:done={i < step}>{name}</li>
      {/each}
    </ol>
  </header>

  {#if loadError}
    <p class="error">Couldn't read your configuration: {loadError}</p>
  {:else if !choices}
    <p class="muted">Loading…</p>
  {:else if step === 0}
    <section>
      <h2>Who writes the summaries?</h2>
      <p class="muted">Each transcript is titled, filed and summarised by a language model.</p>
      <div class="choices" role="radiogroup" aria-label="Summary provider">
        <label class="option" class:selected={choices.llm_provider === 'ollama'}>
          <input type="radio" name="provider" checked={choices.llm_provider === 'ollama'} onchange={() => setProvider('ollama')} />
          <span><strong>Ollama</strong><small>A model running locally. Nothing leaves this Mac.</small></span>
        </label>
        <label class="option" class:selected={choices.llm_provider === 'claude'}>
          <input type="radio" name="provider" checked={choices.llm_provider === 'claude'} onchange={() => setProvider('claude')} />
          <span><strong>Claude</strong><small>Through the Claude Code CLI. Transcripts are sent to Anthropic.</small></span>
        </label>
      </div>

      <label class="field">
        <span>Model</span>
        {#if choices.llm_provider === 'claude'}
          <select bind:value={choices.llm_model} onchange={resetCheck}>
            {#each opts.claude_models as m}<option value={m}>{m}</option>{/each}
          </select>
        {:else}
          <input type="text" bind:value={choices.llm_model} oninput={resetCheck} placeholder={opts.default_ollama_model} spellcheck="false" />
        {/if}
      </label>

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
        <button onclick={check} disabled={checking || !choices.llm_model.trim()}>Check connection</button>
        <button class="primary" onclick={() => (step = 1)} disabled={!checkedOK}>Next</button>
      </footer>
    </section>
  {:else if step === 1}
    <section>
      <h2>How should it transcribe?</h2>
      <p class="muted">Tacit listens to your microphone.</p>

      <label class="field">
        <span>Language</span>
        <select bind:value={choices.language}>
          {#each opts.languages as l}<option value={l.code}>{l.label}</option>{/each}
        </select>
        <small class="muted">Choosing the language you speak cuts wrong-language transcriptions.</small>
      </label>

      <label class="check">
        <input type="checkbox" bind:checked={choices.experimental} />
        <span><strong>Experimental transcription</strong><small>Suppresses non-speech tokens and pads speech onsets.</small></span>
      </label>

      {#if saveError}<pre class="error">{saveError}</pre>{/if}
      <footer>
        <button onclick={() => (step = 0)}>Back</button>
        <button class="primary" onclick={save} disabled={saving}>{saving ? 'Saving…' : 'Save and continue'}</button>
      </footer>
    </section>
  {:else if step === 2}
    <section>
      <h2>Speech model</h2>
      {#if opts.model.present}
        <p class="ok">✓ <code>{opts.model.name}</code> is on this Mac.</p>
      {:else}
        <p class="muted">
          Transcription runs on <code>{opts.model.name}</code>, which has to be downloaded once. It is large — around
          1.6 GB for the default — so a fast connection helps.
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
          <button onclick={cancelDownload}>Cancel</button>
        {:else}
          <button onclick={() => (step = 1)}>Back</button>
          {#if opts.model.present}
            <button class="primary" onclick={goToPermissions}>Next</button>
          {:else}
            <button onclick={goToPermissions}>Skip for now</button>
            <button class="primary" onclick={startDownload}>Download</button>
          {/if}
        {/if}
      </footer>
    </section>
  {:else if step === 3}
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
        <button onclick={() => (step = 2)}>Back</button>
        <button class="primary" onclick={() => (step = 4)}>Next</button>
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
  .pill { padding: 2px 10px; border-radius: 999px; background: var(--accent-soft); font-size: 12px; }
</style>
