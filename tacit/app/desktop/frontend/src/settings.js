// The settings window's only door to Go. Each function calls a method of
// SettingsService (app/desktop/settings.go) by name; a Go test
// (onboarding_test.go) reads this file and checks every name against the
// service, as it does for backend.js.
//
// Under `vite dev` the same functions run against in-memory fakes, so the
// window can be worked on in a browser at /?view=settings.
import { Call } from '@wailsio/runtime'

const svc = 'main.SettingsService.'

const real = {
  load: () => Call.ByName(svc + 'Load'),
  set: (key, value) => Call.ByName(svc + 'Set', key, value),
  clear: (key) => Call.ByName(svc + 'Clear', key),
  restart: () => Call.ByName(svc + 'Restart'),
  editFile: () => Call.ByName(svc + 'EditFile'),
  setUp: () => Call.ByName(svc + 'SetUp'),
}

function makeFake() {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms))
  const defaults = {
    whisper_model: ['large-v3-turbo', 'string'], language: ['auto', 'string'], initial_prompt: ['', 'string'],
    min_speech_duration: ['2s', 'duration'], silence_duration: ['10s', 'duration'],
    speech_threshold: [0.5, 'number'], energy_threshold: [200, 'number'], llm_provider: ['ollama', 'string'],
    llm_model: ['qwen3.5', 'string'], skill_agent: ['claude', 'string'],
    max_segment_duration: ['30s', 'duration'], max_session_duration: ['5m0s', 'duration'],
    transcript_denylist: [[], 'list'], dedup_window: ['3h0m0s', 'duration'], min_char_rate: [0.2, 'number'],
  }
  const overrides = { language: 'ko', silence_duration: '8s' }
  const snapshot = () => ({
    path: '/Users/me/.tacit/config-override.yaml',
    running: true,
    fields: Object.entries(defaults).map(([key, [def, kind]]) => ({
      key, kind, default: def, overridden: key in overrides, value: key in overrides ? overrides[key] : def,
    })),
  })

  return {
    async load() { return snapshot() },
    async set(key, value) {
      await wait(150)
      if (defaults[key][1] === 'duration' && !/^\d+(\.\d+)?(ns|us|µs|ms|s|m|h)+/.test(value)) {
        throw new Error(`${key}: time: missing unit in duration "${value}"`)
      }
      overrides[key] = value
      return snapshot()
    },
    async clear(key) { await wait(150); delete overrides[key]; return snapshot() },
    async restart() { await wait(800) },
    async editFile() { console.log('editFile') },
    async setUp() { console.log('setUp') },
  }
}

export const settings = import.meta.env.DEV ? makeFake() : real
