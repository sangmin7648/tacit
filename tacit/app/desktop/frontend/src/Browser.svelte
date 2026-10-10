<script>
  import { onMount } from 'svelte'
  import { knowledge } from './knowledge.js'
  import { errorText } from './backend.js'

  const RANGES = [
    { days: 1, label: 'Today' },
    { days: 7, label: 'Last 7 days' },
    { days: 30, label: 'Last 30 days' },
    { days: 0, label: 'Everything' },
  ]

  let query = $state('')
  let days = $state(7)
  let category = $state('') // '' = all
  let entries = $state([])
  let loading = $state(true)
  let error = $state('')
  let selectedPath = $state('')
  let detail = $state(null)
  let detailError = $state('')

  const categories = $derived([...new Set(entries.map((e) => e.category))].sort())
  const shown = $derived(category ? entries.filter((e) => e.category === category) : entries)

  // Each load gets a number; a slow search that finishes after a newer one
  // must not overwrite it.
  let seq = 0
  async function load() {
    const mine = ++seq
    loading = true
    error = ''
    try {
      const result = await knowledge.search(query, days)
      if (mine !== seq) return
      entries = result
      if (category && !result.some((e) => e.category === category)) category = ''
      if (!selectedPath && result.length) select(result[0].path)
    } catch (e) {
      if (mine === seq) error = errorText(e)
    } finally {
      if (mine === seq) loading = false
    }
  }

  // Typing searches after a pause rather than on every key: search runs rg
  // over the whole knowledge base.
  let debounce
  function onQuery() {
    clearTimeout(debounce)
    debounce = setTimeout(load, 250)
  }

  async function select(path) {
    selectedPath = path
    detailError = ''
    try {
      const e = await knowledge.get(path)
      if (selectedPath === path) detail = e
    } catch (e) {
      if (selectedPath === path) {
        detail = null
        detailError = errorText(e)
      }
    }
  }

  onMount(() => {
    // Opened from the menu's Recent list: show that note, not the newest.
    knowledge.takePending().then((path) => path && select(path)).finally(load)
    // A note stored while the window is open shows up without a refresh.
    const offStored = knowledge.onStored(() => load())
    const offSelect = knowledge.onSelect((path) => select(path))
    return () => {
      offStored()
      offSelect()
    }
  })

  const fmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  const when = (iso) => fmt.format(new Date(iso))
</script>

<div class="browser">
  <aside>
    <div class="filters">
      <input
        type="search"
        placeholder="Search notes"
        bind:value={query}
        oninput={onQuery}
        spellcheck="false"
        aria-label="Search notes"
      />
      <div class="row">
        <select bind:value={days} onchange={load} aria-label="Time range">
          {#each RANGES as r}<option value={r.days}>{r.label}</option>{/each}
        </select>
        <select bind:value={category} aria-label="Category">
          <option value="">All categories</option>
          {#each categories as c}<option value={c}>{c}</option>{/each}
        </select>
      </div>
      <p class="count muted">
        {#if loading}Loading…{:else}{shown.length} {shown.length === 1 ? 'note' : 'notes'}{/if}
      </p>
    </div>

    {#if error}
      <pre class="error pad">{error}</pre>
    {:else if !loading && shown.length === 0}
      <p class="muted pad">{query ? 'No notes match.' : 'No notes in this range yet.'}</p>
    {/if}

    <ul class="list">
      {#each shown as e (e.path)}
        <li>
          <button class="item" class:selected={e.path === selectedPath} onclick={() => select(e.path)}>
            <span class="title">{e.title}</span>
            <span class="meta">{e.category} · {when(e.created_at)}</span>
            {#if e.match_lines.length}
              <span class="snippet">{e.match_lines[0]}</span>
            {:else if e.summary}
              <span class="snippet">{e.summary}</span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
  </aside>

  <article>
    {#if detailError}
      <pre class="error">{detailError}</pre>
    {:else if detail}
      <header>
        <h1>{detail.title}</h1>
        <p class="muted">{detail.category} · {when(detail.created_at)}</p>
        {#if detail.keywords?.length}
          <p class="chips">{#each detail.keywords as k}<span class="chip">{k}</span>{/each}</p>
        {/if}
        <p class="actions">
          <button onclick={() => knowledge.open(detail.path)}>Open</button>
          <button onclick={() => knowledge.reveal(detail.path)}>Show in Finder</button>
        </p>
      </header>
      {#if detail.summary}
        <h2>Summary</h2>
        <p class="text">{detail.summary}</p>
      {/if}
      {#if detail.content}
        <h2>Transcript</h2>
        <p class="text">{detail.content}</p>
      {/if}
    {:else if !loading && shown.length}
      <p class="muted">Select a note.</p>
    {/if}
  </article>
</div>

<style>
  .browser {
    display: grid;
    grid-template-columns: minmax(240px, 34%) 1fr;
    height: 100vh;
  }
  aside {
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--line);
    min-height: 0;
  }
  .filters { padding: 12px; display: grid; gap: 8px; border-bottom: 1px solid var(--line); }
  .row { display: flex; gap: 6px; }
  .row select { flex: 1; min-width: 0; }
  input[type='search'] {
    font: inherit;
    padding: 6px 8px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel);
    color: var(--text);
  }
  .count { margin: 0; font-size: 12px; }
  .pad { padding: 12px; margin: 0; }
  .list { list-style: none; margin: 0; padding: 0; overflow-y: auto; flex: 1; }
  .item {
    display: grid;
    gap: 2px;
    width: 100%;
    text-align: left;
    border: none;
    border-bottom: 1px solid var(--line);
    border-radius: 0;
    padding: 10px 12px;
    background: transparent;
  }
  .item.selected { background: var(--panel); box-shadow: inset 3px 0 0 var(--accent); }
  .title { font-weight: 600; }
  .meta { font-size: 12px; color: var(--muted); }
  .snippet {
    font-size: 12px;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  article {
    overflow-y: auto;
    padding: 20px 28px;
    -webkit-user-select: text;
    user-select: text;
  }
  article h1 { font-size: 20px; margin: 0 0 4px; }
  article h2 { font-size: 13px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); margin: 20px 0 6px; }
  article header p { margin: 4px 0; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .chip { font-size: 12px; padding: 1px 8px; border-radius: 10px; border: 1px solid var(--line); }
  .actions { display: flex; gap: 6px; margin-top: 10px !important; }
  .text { white-space: pre-wrap; line-height: 1.6; margin: 0; }
</style>
