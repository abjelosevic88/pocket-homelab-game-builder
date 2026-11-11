// README screenshots and a browser smoke test against a demo instance:
//
//   make build && DATA_DIR=/tmp/pgb-demo DEMO_DATA=true AUTH_MODE=none HOST=127.0.0.1 PORT=18275 ./pocket-gamebuilder &
//   CDP=http://127.0.0.1:9345 BASE=http://127.0.0.1:18275 node scripts/screenshots.mjs   # any Chrome with remote debugging
//   SHOTS=0 … node scripts/screenshots.mjs                                                # checks only
// playwright-core comes from web/node_modules, or set PW_FROM to a package.json next to an install of it.
import { createRequire } from 'node:module'
import assert from 'node:assert/strict'

const require = createRequire(process.env.PW_FROM || new URL('../web/package.json', import.meta.url))
let pw
try { pw = require('playwright') } catch { pw = require('playwright-core') }
const { chromium } = pw
const base = process.env.BASE || 'http://127.0.0.1:18275'
const shots = process.env.SHOTS !== '0'
const dir = process.env.OUT || new URL('../docs/screenshots/', import.meta.url).pathname
const browser = process.env.CDP ? await chromium.connectOverCDP(process.env.CDP) : await chromium.launch()
const errors = []

async function session({ phone = false, dark = false, w = 1440, h = 900 } = {}) {
  const ctx = await browser.newContext({
    viewport: phone ? { width: 390, height: 844 } : { width: w, height: h },
    deviceScaleFactor: 2, isMobile: phone, hasTouch: phone, colorScheme: dark ? 'dark' : 'light',
  })
  const page = await ctx.newPage()
  page.on('pageerror', e => errors.push(`pageerror: ${e}`))
  page.on('console', m => { if (m.type() === 'error' && !/Failed to load resource|ERR_/.test(m.text())) errors.push(m.text()) })
  return page
}
async function go(page, path, wait = 1600) { await page.goto(base + path, { waitUntil: 'domcontentloaded' }); await page.waitForTimeout(wait) }
async function shot(page, name) { if (shots) await page.screenshot({ path: dir + name + '.png' }) }
const api = (page, path) => page.evaluate(p => fetch('/api/v1' + p).then(r => r.json()), path)
const has = async (page, text, what) => assert.ok((await page.content()).includes(text), what)

const p = await session()
await go(p, '/', 5000) // covers are drawn the first time
await go(p, '/', 1500)
await has(p, 'Kitty', 'the games')
await shot(p, 'desktop-games')

const games = (await api(p, '/games')).games
const jump = games.find(g => g.template === 'platformer')
const maze = games.find(g => g.template === 'maze')
await go(p, `/games/${jump.id}`, 2200)
await has(p, 'Best scores', 'the game page')
await p.click('.play-box .gb-go')
await p.waitForTimeout(250)
await p.keyboard.down('ArrowRight'); await p.waitForTimeout(700); await p.keyboard.press('Space'); await p.waitForTimeout(450); await p.keyboard.up('ArrowRight')
await shot(p, 'desktop-game')

await go(p, `/games/${jump.id}/edit`, 2200)
await has(p, 'What it does', 'the editor')
await p.click('.pal-item >> text=Snail')
await p.waitForTimeout(400)
await shot(p, 'desktop-editor')
await p.click('.ed-tabs >> text=Rules'); await p.waitForTimeout(600)
await shot(p, 'desktop-rules')

// a sprite, drawn
await go(p, '/library', 1500)
const robot = (await api(p, '/assets?kind=image')).assets.find(a => a.name === 'Robot')
assert.ok(robot, 'the drawn robot is there')
await p.locator('.asset-card', { hasText: 'Robot' }).locator('.asset-pic').click()
await p.waitForTimeout(900)
await shot(p, 'desktop-pixels')
await p.keyboard.press('Escape')

await go(p, '/', 1200)
await p.click('.sidebar .create-btn'); await p.waitForTimeout(800)
await shot(p, 'desktop-new')
await p.keyboard.press('Escape')

await go(p, `/p/${maze.share_key}`, 2200)
await has(p, 'Best scores', 'the shared page')
await shot(p, 'desktop-public')

// one HTML file that plays on its own
const html = await p.evaluate(id => fetch(`/api/v1/games/${id}/export.html`).then(r => r.text()), jump.id)
assert.ok(html.includes('window.__GAME__') && html.length > 20000, 'the HTML file')

const m = await session({ phone: true })
await go(m, '/', 2000)
await shot(m, 'phone-games')
await go(m, `/games/${jump.id}/edit`, 2200)
await shot(m, 'phone-editor')
const d = await session({ phone: true, dark: true })
await go(d, `/p/${jump.share_key}`, 2000)
await d.tap('.gb-go')
await d.waitForTimeout(1200)
await shot(d, 'phone-play-dark')

await browser.close()
if (errors.length) { console.error(errors.join('\n')); process.exit(1) }
console.log('ok')
