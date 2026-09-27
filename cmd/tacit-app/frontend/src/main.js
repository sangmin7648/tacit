import { mount } from 'svelte'
import App from './App.svelte'
import Browser from './Browser.svelte'
import './app.css'

// One bundle serves both windows; the Go side opens the notes browser at
// /?view=browser (see KnowledgeService.show).
const browser = new URLSearchParams(location.search).get('view') === 'browser'
if (browser) document.title = 'Tacit Notes'
mount(browser ? Browser : App, { target: document.getElementById('app') })
