// Renders card.html to social.png, the 1280x640 image for the repository's social preview
// (Settings > General > Social preview).
//   npm install && npm run build
// CHROME_PATH overrides the Chrome binary.
import { chromium } from 'playwright-core'
import { fileURLToPath } from 'url'
import path from 'path'

const dir = path.dirname(fileURLToPath(import.meta.url))
const chrome = process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'

const browser = await chromium.launch({ executablePath: chrome, headless: true })
const page = await (await browser.newContext({ viewport: { width: 1280, height: 640 }, deviceScaleFactor: 1 })).newPage()
await page.goto('file://' + path.join(dir, 'card.html'))
await page.waitForFunction(() => [...document.images].every(i => i.complete))
await page.screenshot({ path: path.join(dir, 'social.png') })
await browser.close()
console.log('wrote', path.relative(process.cwd(), path.join(dir, 'social.png')))
