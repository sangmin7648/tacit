<script>
  import { onMount } from 'svelte'
  import { knowledge } from './knowledge.js'
  import { errorText } from './backend.js'

  const RANGES = [
    { days: 1, label: 'Today' },
    { days: 7, label: '7 days' },
    { days: 30, label: '30 days' },
    { days: 0, label: 'All' },
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
  const groups = $derived(groupByDay(shown))
  let searchEl

  function dayLabel(d) {
    const startOfDay = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
    const diff = Math.round((startOfDay(new Date()) - startOfDay(d)) / 86_400_000)
    if (diff <= 0) return 'Today'
    if (diff === 1) return 'Yesterday'
    return new Intl.DateTimeFormat(undefined, { weekday: 'long', month: 'short', day: 'numeric' }).format(d)
  }

  // groupByDay keeps the order the list came in, which is newest first when
  // listing and best match first when searching; a search is not grouped.
  function groupByDay(list) {
    if (query.trim()) return [{ label: `${list.length} ${list.length === 1 ? 'match' : 'matches'}`, items: list }]
    const out = []
    for (const e of list) {
      const label = dayLabel(new Date(e.created_at))
      if (out.at(-1)?.label !== label) out.push({ label, items: [] })
      out.at(-1).items.push(e)
    }
    return out
  }

  // marks splits text into parts, flagging those the query matched. The
  // query is a regular expression for search, so one that does not compile
  // highlights nothing.
  function marks(text) {
    const q = query.trim()
    if (!q || !text) return [{ text, hit: false }]
    let re
    try {
      re = new RegExp(`(${q})`, 'gi')
    } catch {
      return [{ text, hit: false }]
    }
    return text.split(re).map((part, i) => ({ text: part, hit: i % 2 === 1 })).filter((m) => m.text)
  }

  const paragraphs = (t) => (t ?? '').split(/\n{2,}/).map((p) => p.trim()).filter(Boolean)

  let copied = $state(false)
  async function copyTranscript() {
    try {
      await navigator.clipboard.writeText(detail.content)
      copied = true
      setTimeout(() => (copied = false), 1500)
    } catch {}
  }

  function move(step) {
    if (!shown.length) return
    const i = shown.findIndex((e) => e.path === selectedPath)
    const next = shown[Math.min(shown.length - 1, Math.max(0, i + step))]
    select(next.path)
    document.querySelector(`[data-path="${CSS.escape(next.path)}"]`)?.scrollIntoView({ block: 'nearest' })
  }

  function onKey(e) {
    if (e.key === 'ArrowDown') { e.preventDefault(); move(1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); move(-1) }
    else if ((e.metaKey || e.ctrlKey) && e.key === 'f') { e.preventDefault(); searchEl?.focus(); searchEl?.select() }
    else if (e.key === 'Escape' && query) { query = ''; load() }
  }

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
      const gone = !result.some((e) => e.path === selectedPath)
      if (result.length && (!selectedPath || (query.trim() && gone))) select(result[0].path)
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
  const timeFmt = new Intl.DateTimeFormat(undefined, { timeStyle: 'short' })
  const timeOf = (iso) => timeFmt.format(new Date(iso))
</script>

<svelte:window onkeydown={onKey} />

<div class="browser">
  <aside>
    <div class="filters">
      <input
        type="search"
        placeholder="Search notes  ⌘F"
        bind:this={searchEl}
        bind:value={query}
        oninput={onQuery}
        spellcheck="false"
        aria-label="Search notes"
      />
      <div class="segments" role="group" aria-label="Time range">
        {#each RANGES as r}
          <button class:on={days === r.days} onclick={() => { days = r.days; load() }}>{r.label}</button>
        {/each}
      </div>
      {#if categories.length > 1}
        <select bind:value={category} aria-label="Category">
          <option value="">All categories</option>
          {#each categories as c}<option value={c}>{c}</option>{/each}
        </select>
      {/if}
    </div>

    {#if error}
      <pre class="error pad">{error}</pre>
    {:else if loading && !entries.length}
      <p class="muted pad">Loading…</p>
    {:else if shown.length === 0}
      <div class="empty">
        <p><strong>{query ? 'No notes match' : 'Nothing here yet'}</strong></p>
        <p class="muted">
          {query ? 'Try a different word, or widen the time range.' : 'Notes appear here as Tacit hears and files what you say.'}
        </p>
        {#if days !== 0}<button onclick={() => { days = 0; load() }}>Show all time</button>{/if}
      </div>
    {/if}

    <div class="list">
      {#each groups as g (g.label)}
        <h3>{g.label}</h3>
        <ul>
          {#each g.items as e (e.path)}
            <li>
              <button class="item" data-path={e.path} class:selected={e.path === selectedPath} onclick={() => select(e.path)}>
                <span class="title">{#each marks(e.title) as m}{#if m.hit}<mark>{m.text}</mark>{:else}{m.text}{/if}{/each}</span>
                <span class="snippet">
                  {#each marks(e.match_lines[0] ?? e.summary) as m}{#if m.hit}<mark>{m.text}</mark>{:else}{m.text}{/if}{/each}
                </span>
                <span class="meta">
                  <span class="tag">{e.category}</span>
                  <span>{timeOf(e.created_at)}</span>
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/each}
    </div>
  </aside>

  <article>
    {#if detailError}
      <pre class="error">{detailError}</pre>
    {:else if detail}
      <div class="page">
        <header>
          <p class="kicker">
            <span class="tag">{detail.category}</span>
            <span class="muted">{when(detail.created_at)}</span>
          </p>
          <h1>{detail.title}</h1>
          {#if detail.keywords?.length}
            <p class="chips">{#each detail.keywords as k}<span class="chip">{k}</span>{/each}</p>
          {/if}
          <p class="actions">
            <button onclick={() => knowledge.reveal(detail.path)}>Show in Finder</button>
            <button onclick={() => knowledge.open(detail.path)}>Open</button>
          </p>
        </header>
        {#if detail.summary}
          <div class="summary">
            <h2>Summary</h2>
            <p>{#each marks(detail.summary) as m}{#if m.hit}<mark>{m.text}</mark>{:else}{m.text}{/if}{/each}</p>
          </div>
        {/if}
        {#if detail.content && detail.content.trim() !== detail.summary?.trim()}
          <div class="transcript">
            <div class="section-head">
              <h2>Transcript</h2>
              <button class="link" onclick={copyTranscript}>{copied ? 'Copied' : 'Copy'}</button>
            </div>
            {#each paragraphs(detail.content) as para}
              <p>{#each marks(para) as m}{#if m.hit}<mark>{m.text}</mark>{:else}{m.text}{/if}{/each}</p>
            {/each}
          </div>
        {/if}
      </div>
    {:else if !loading && shown.length}
      <p class="muted blank">Select a note.</p>
    {/if}
  </article>
</div>

<style>
  .browser {
    display: grid;
    grid-template-columns: minmax(260px, 34%) 1fr;
    height: 100vh;
  }
  aside {
    display: flex;
    flex-direction: column;
    background: var(--sunken);
    border-right: 1px solid var(--line);
    min-height: 0;
  }
  .filters { padding: 12px 12px 10px; display: grid; gap: 8px; }
  input[type='search'] { background: var(--panel); }
  .segments {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 1fr;
    padding: 2px;
    border-radius: 8px;
    background: color-mix(in srgb, var(--text) 8%, transparent);
  }
  .segments button {
    border: none;
    background: transparent;
    box-shadow: none;
    padding: 3px 4px;
    font-size: 12px;
    color: var(--muted);
  }
  .segments button.on { background: var(--panel); color: var(--text); font-weight: 600; box-shadow: var(--shadow); }
  .pad { padding: 12px; margin: 0; }
  .empty { padding: 28px 18px; text-align: center; display: grid; gap: 4px; justify-items: center; }
  .empty p { margin: 0; }
  .list { flex: 1; overflow-y: auto; padding: 0 8px 12px; }
  .list h3 {
    margin: 12px 6px 4px;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--muted);
  }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
  .item {
    display: grid;
    gap: 1px;
    width: 100%;
    text-align: left;
    border: none;
    box-shadow: none;
    border-radius: 8px;
    padding: 8px 10px;
    background: transparent;
  }
  .item:hover:not(.selected) { background: color-mix(in srgb, var(--text) 6%, transparent); }
  .item.selected { background: var(--accent); color: var(--accent-text); }
  .title { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .snippet {
    font-size: 12px;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .item.selected .snippet, .item.selected .meta { color: inherit; opacity: 0.85; }
  .meta { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--muted); margin-top: 2px; }
  .tag {
    padding: 0 7px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 600;
    color: var(--muted);
    background: color-mix(in srgb, var(--text) 8%, transparent);
  }
  .item.selected .tag { background: rgba(255, 255, 255, 0.25); color: inherit; }
  mark { background: color-mix(in srgb, #ffcc00 55%, transparent); color: inherit; border-radius: 2px; }
  .item.selected mark { background: rgba(255, 255, 255, 0.3); }

  article {
    overflow-y: auto;
    padding: 28px 36px 40px;
    background: var(--panel);
    -webkit-user-select: text;
    user-select: text;
  }
  .page { max-width: 640px; margin: 0 auto; }
  .blank { text-align: center; margin-top: 30vh; }
  .kicker { display: flex; align-items: center; gap: 8px; margin: 0 0 6px; font-size: 12px; }
  article h1 { font-size: 26px; line-height: 1.25; margin: 0 0 10px; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; margin: 0 0 12px; }
  .chip { font-size: 12px; padding: 1px 9px; border-radius: 999px; background: var(--sunken); color: var(--muted); }
  .actions { display: flex; gap: 6px; margin: 0 0 22px; }
  .summary {
    padding: 12px 16px;
    margin-bottom: 22px;
    border-radius: 10px;
    background: var(--accent-soft);
  }
  article h2 {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    margin: 0 0 6px;
  }
  .summary p { margin: 0; font-size: 14.5px; line-height: 1.6; }
  .section-head { display: flex; justify-content: space-between; align-items: baseline; }
  .transcript p { font-size: 14.5px; line-height: 1.75; margin: 0 0 12px; }
  button.link { border: none; box-shadow: none; background: none; padding: 0; color: var(--accent); font-size: 12px; }
</style>
