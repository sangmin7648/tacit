// Renders scene.html frame by frame in Chrome and encodes ../demo.gif with ffmpeg.
//   npm install && npm run build
//   node render.mjs 2,6.5   # preview PNGs at those seconds, written to ./preview
// CHROME_PATH overrides the Chrome binary.
import { chromium } from 'playwright-core'
import { spawnSync } from 'child_process'
import { fileURLToPath } from 'url'
import path from 'path'
import fs from 'fs'
import os from 'os'

const dir = path.dirname(fileURLToPath(import.meta.url))
const FPS = 25
const chrome = process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const previewAt = process.argv[2]?.split(',').map(Number)

const browser = await chromium.launch({ executablePath: chrome, headless: true })
const page = await (await browser.newContext({ viewport: { width: 800, height: 480 }, deviceScaleFactor: 2 })).newPage()
await page.goto('file://' + path.join(dir, 'scene.html'))
await page.waitForFunction(() => [...document.images].every(i => i.complete))
await page.evaluate(() => document.fonts.ready)
const duration = await page.evaluate(() => window.DURATION)

const shoot = async (t, file) => {
  await page.evaluate(t => window.render(t), t)
  await page.screenshot({ path: file })
}

if (previewAt) {
  const out = path.join(dir, 'preview')
  fs.mkdirSync(out, { recursive: true })
  for (const t of previewAt) await shoot(t, path.join(out, `t${t}.png`))
} else {
  const frames = fs.mkdtempSync(path.join(os.tmpdir(), 'tacit-demo-'))
  const count = Math.round(FPS * duration)
  for (let i = 0; i < count; i++) await shoot(i / FPS, path.join(frames, `f${String(i).padStart(4, '0')}.png`))
  const filter = 'scale=800:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle'
  const gif = path.join(dir, '..', 'demo.gif')
  const r = spawnSync('ffmpeg', ['-y', '-loglevel', 'error', '-framerate', String(FPS), '-i', path.join(frames, 'f%04d.png'), '-vf', filter, '-loop', '0', gif], { stdio: 'inherit' })
  fs.rmSync(frames, { recursive: true, force: true })
  if (r.status !== 0) process.exit(r.status ?? 1)
  console.log('wrote', path.relative(process.cwd(), gif))
}
await browser.close()
