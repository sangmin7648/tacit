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

const real = {
  options: () => Call.ByName(svc + 'Options'),
  checkProvider: (choices) => Call.ByName(svc + 'CheckProvider', choices),
  apply: (choices) => Call.ByName(svc + 'Apply', choices),
  // Returns a cancellable promise: cancel() stops the download in Go.
  downloadModel: () => Call.ByName(svc + 'DownloadModel'),
  permissions: () => Call.ByName(svc + 'Permissions'),
  requestMicrophone: () => Call.ByName(svc + 'RequestMicrophone'),
  openPrivacySettings: (pane) => Call.ByName(svc + 'OpenPrivacySettings', pane),
  finish: (startListening) => Call.ByName(svc + 'Finish', startListening),
  onModelProgress: (fn) => Events.On(MODEL_PROGRESS, (e) => fn(e.data)),
}

function makeFake() {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms))
  const perms = { microphone: 'undetermined' }
  let progressFn = () => {}
  let modelPresent = false

  return {
    async options() {
      return {
        configured: false,
        choices: {
          llm_provider: 'ollama', llm_model: 'qwen3.5', skill_agent: 'claude',
          language: 'auto', experimental: false,
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
    async checkProvider(c) {
      await wait(400)
      if (c.llm_provider === 'ollama' && c.llm_model !== 'qwen3.5') {
        throw new Error(`Ollama model "${c.llm_model}" not found\n  → Pull it with: ollama pull ${c.llm_model}`)
      }
    },
    async apply() {
      await wait(200)
      return { override_path: '~/.tacit/config-override.yaml', reference_path: '~/.tacit/config.yaml', installed_skills: [] }
    },
    downloadModel() {
      const total = 1_620_000_000
      let done = 0
      let timer
      let reject
      const p = new Promise((resolve, rej) => {
        reject = rej
        timer = setInterval(() => {
          done = Math.min(total, done + total / 40)
          progressFn({ done, total })
          if (done >= total) {
            clearInterval(timer)
            modelPresent = true
            resolve()
          }
        }, 100)
      })
      p.cancel = () => { clearInterval(timer); reject(new Error('download cancelled')) }
      return p
    },
    async permissions() { return { ...perms } },
    async requestMicrophone() { await wait(300); perms.microphone = 'granted' },
    async openPrivacySettings() { await wait(300); perms.microphone = 'granted' },
    async finish(startListening) { console.log('finish', { startListening }) },
    onModelProgress(fn) { progressFn = fn; return () => { progressFn = () => {} } },
  }
}

export const backend = import.meta.env.DEV ? makeFake() : real

// errorText turns a rejected call into something to show: Go errors arrive as
// a RuntimeError carrying the Go message.
export function errorText(err) {
  return (err && err.message) || String(err)
}
