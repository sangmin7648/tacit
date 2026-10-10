// The onboarding window's only door to Go. Each function calls a method of
// OnboardingService (app/desktop/onboarding.go) by name; names are plain
// strings, so a Go test (onboarding_test.go) reads this file and checks every
// one of them — and MODEL_PROGRESS — against the service.
//
// Under `vite dev` there is no Go side, so the same functions run against the
// in-memory fakes below: that is how the window is worked on in a browser.
import { Call, Events } from '@wailsio/runtime'

const svc = 'main.OnboardingService.'
export const MODEL_PROGRESS = 'onboarding:model-progress'
export const PULL_PROGRESS = 'onboarding:pull-progress'

const real = {
  options: () => Call.ByName(svc + 'Options'),
  recommend: () => Call.ByName(svc + 'Recommend'),
  checkProvider: (choices) => Call.ByName(svc + 'CheckProvider', choices),
  apply: (choices) => Call.ByName(svc + 'Apply', choices),
  // Returns a cancellable promise, like downloadModel.
  pullOllamaModel: (model) => Call.ByName(svc + 'PullOllamaModel', model),
  // Returns a cancellable promise: cancel() stops the download in Go.
  downloadModel: () => Call.ByName(svc + 'DownloadModel'),
  permissions: () => Call.ByName(svc + 'Permissions'),
  requestMicrophone: () => Call.ByName(svc + 'RequestMicrophone'),
  openPrivacySettings: (pane) => Call.ByName(svc + 'OpenPrivacySettings', pane),
  finish: (startListening) => Call.ByName(svc + 'Finish', startListening),
  onModelProgress: (fn) => Events.On(MODEL_PROGRESS, (e) => fn(e.data)),
  onPullProgress: (fn) => Events.On(PULL_PROGRESS, (e) => fn(e.data)),
}

// A cancellable fake of a long call that reports progress, for `vite dev`.
function fakeTransfer(total, report, onDone) {
  let done = 0
  let timer
  let reject
  const p = new Promise((resolve, rej) => {
    reject = rej
    timer = setInterval(() => {
      done = Math.min(total, done + total / 40)
      report({ done, total })
      if (done >= total) {
        clearInterval(timer)
        onDone()
        resolve()
      }
    }, 100)
  })
  p.cancel = () => { clearInterval(timer); reject(new Error('download cancelled')) }
  return p
}

function makeFake() {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms))
  const perms = { microphone: 'undetermined' }
  let progressFn = () => {}
  let pullFn = () => {}
  let modelPresent = false
  let ollamaHasModel = false

  return {
    async options() {
      return {
        configured: false,
        choices: {
          llm_provider: 'ollama', llm_model: 'qwen3.5', skill_agent: 'claude',
          language: 'auto', whisper_model: 'large-v3-turbo', experimental: false,
        },
        providers: ['ollama', 'claude'],
        claude_models: ['haiku', 'sonnet', 'opus'],
        languages: [
          { code: 'auto', label: 'auto (detect)' },
          { code: 'en', label: 'english' },
          { code: 'ko', label: 'korean' },
        ],
        default_ollama_model: 'qwen3.5',
        model: { name: 'large-v3-turbo', present: modelPresent },
      }
    },
    async recommend() {
      await wait(500)
      return {
        choices: {
          llm_provider: 'ollama', llm_model: 'qwen3.5', skill_agent: 'claude',
          language: 'auto', whisper_model: 'large-v3-turbo', experimental: false,
        },
        reasons: {
          llm_provider: 'Ollama is running here, so summaries stay on this Mac.',
          llm_model: 'qwen3.5 gave the best titles and categories in our testing. It is not installed yet; download it below.',
          skill_agent: 'Claude Code is installed, so your notes can be searched from it.',
          language: 'Your Mac lists en and ko, so Tacit detects the language as you speak. Pick one if you only speak that.',
          whisper_model: 'This Mac has 32 GB of RAM. large-v3-turbo uses about 2.1 GB of RAM while transcribing and is the most accurate choice that fits.',
        },
        claude_available: true,
        ollama: { installed: true, running: true, models: ollamaHasModel ? ['qwen3.5:latest', 'llama3.2:latest'] : ['llama3.2:latest'] },
        agents: [{ name: 'claude', label: 'Claude Code', installed: true, recommended: true }],
        whisper_models: [
          { name: 'base', download_mb: 142, ram_mb: 388 },
          { name: 'small', download_mb: 466, ram_mb: 852 },
          { name: 'large-v3-turbo', download_mb: 1600, ram_mb: 2100, recommended: true, installed: modelPresent },
        ].map((m) => ({ installed: false, recommended: false, ...m })),
        memory_gb: 32,
      }
    },
    pullOllamaModel() {
      return fakeTransfer(5_000_000_000, (p) => pullFn(p), () => { ollamaHasModel = true })
    },
    async checkProvider(c) {
      await wait(400)
      if (c.llm_provider === 'ollama' && !(ollamaHasModel && c.llm_model === 'qwen3.5') && c.llm_model !== 'llama3.2') {
        throw new Error(`Ollama model "${c.llm_model}" not found\n  → Pull it with: ollama pull ${c.llm_model}`)
      }
    },
    async apply() {
      await wait(200)
      return { override_path: '~/.tacit/config-override.yaml', reference_path: '~/.tacit/config.yaml', installed_skills: [] }
    },
    downloadModel() {
      return fakeTransfer(1_620_000_000, (p) => progressFn(p), () => { modelPresent = true })
    },
    async permissions() { return { ...perms } },
    async requestMicrophone() { await wait(2500); perms.microphone = 'granted' },
    async openPrivacySettings() { await wait(300); perms.microphone = 'granted' },
    async finish(startListening) { console.log('finish', { startListening }) },
    onModelProgress(fn) { progressFn = fn; return () => { progressFn = () => {} } },
    onPullProgress(fn) { pullFn = fn; return () => { pullFn = () => {} } },
  }
}

export const backend = import.meta.env.DEV ? makeFake() : real

// errorText turns a rejected call into something to show: Go errors arrive as
// a RuntimeError carrying the Go message.
export function errorText(err) {
  return (err && err.message) || String(err)
}
