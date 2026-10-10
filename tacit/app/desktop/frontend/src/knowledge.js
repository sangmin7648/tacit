// The notes browser's only door to Go. Each function calls a method of
// KnowledgeService (app/desktop/knowledge.go) by name; a Go test
// (onboarding_test.go) reads this file and checks every name — and
// STORED — against the service, as it does for backend.js.
//
// Under `vite dev` the same functions run against in-memory fakes, so the
// window can be worked on in a browser at /?view=browser.
import { Call, Events } from '@wailsio/runtime'

const svc = 'main.KnowledgeService.'
export const STORED = 'knowledge:stored'
export const SELECT = 'knowledge:select'

const real = {
  list: (days) => Call.ByName(svc + 'List', days),
  search: (pattern, days) => Call.ByName(svc + 'Search', pattern, days),
  get: (path) => Call.ByName(svc + 'Get', path),
  takePending: () => Call.ByName(svc + 'TakePending'),
  open: (path) => Call.ByName(svc + 'Open', path),
  reveal: (path) => Call.ByName(svc + 'Reveal', path),
  onStored: (fn) => Events.On(STORED, (e) => fn(e.data)),
  onSelect: (fn) => Events.On(SELECT, (e) => fn(e.data)),
}

function makeFake() {
  const day = 86_400_000
  const now = Date.now()
  const samples = [
    ['work', '검색 랭킹 개선 회의', '클릭률 기반 재정렬을 먼저 실험하고 오프라인 지표는 NDCG로 본다.', ['검색', '랭킹', 'NDCG']],
    ['dev', 'Go에서 goroutine leak 방지', 'context 취소 전파와 defer cancel, done 채널 닫기를 함께 쓴다.', ['Go', 'goroutine', 'context']],
    ['daily', '점심 메뉴 고민', '김치찌개와 편의점 사이에서 결정을 못 했다.', ['점심']],
    ['learning', '다익스트라 알고리즘', '우선순위 큐로 O(E log V). 음수 가중치면 벨만-포드.', ['알고리즘', '그래프']],
    ['work', 'Payment module sprint goals', 'API design by me, frontend integration by Kim, PRs due Thursday.', ['sprint', 'payments']],
    ['journal', '발표 회고', '준비가 부족했다. 다음에는 리허설을 두 번 한다.', ['발표', '회고']],
  ]
  const entries = samples.map(([category, title, summary, keywords], i) => ({
    title, category, summary, keywords,
    created_at: new Date(now - i * day * 1.7).toISOString(),
    path: `/Users/me/.tacit/${category}/2026092${i}-10${i}000.md`,
    match_lines: [],
  }))
  let storedFn = () => {}
  const inRange = (e, days) => days <= 0 || now - Date.parse(e.created_at) <= days * day

  return {
    async list(days) { return entries.filter((e) => inRange(e, days)) },
    async search(pattern, days) {
      if (!pattern.trim()) return this.list(days)
      let re
      try { re = new RegExp(pattern, 'i') } catch (e) { throw new Error(`compiling pattern: ${e.message}`) }
      return entries
        .filter((e) => inRange(e, days) && re.test(`${e.title}\n${e.summary}`))
        .map((e) => ({ ...e, match_lines: [e.summary] }))
    },
    async get(path) {
      const e = entries.find((x) => x.path === path)
      return { ...e, content: `${e.summary}\n\n(원문 전사 내용이 여기에 표시됩니다.)`, path }
    },
    async takePending() { return '' },
    onSelect() { return () => {} },
    async open(path) { console.log('open', path) },
    async reveal(path) { console.log('reveal', path) },
    onStored(fn) {
      storedFn = fn
      // Simulate the daemon storing a note while the window is open.
      window.__fakeStore = () => {
        const e = { title: 'Live note from the daemon', category: 'idea', summary: 'Just stored.', keywords: [], created_at: new Date().toISOString(), path: '/Users/me/.tacit/idea/live.md', match_lines: [] }
        entries.unshift(e)
        storedFn(e)
      }
      return () => { storedFn = () => {} }
    },
  }
}

export const knowledge = import.meta.env.DEV ? makeFake() : real
