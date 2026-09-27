import { mount } from 'svelte'
import App from './App.svelte'
import Browser from './Browser.svelte'
import Settings from './Settings.svelte'
import './app.css'

// One bundle serves every window; the Go side opens the others at
// /?view=<name> (see KnowledgeService.show and SettingsService.show).
const views = {
  browser: [Browser, 'Tacit Notes'],
  settings: [Settings, 'Tacit Settings'],
}
const [view, title] = views[new URLSearchParams(location.search).get('view')] ?? [App]
if (title) document.title = title
mount(view, { target: document.getElementById('app') })
