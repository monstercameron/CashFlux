import { createRequire } from 'module';
import path from 'path';
import { fileURLToPath } from 'url';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');
const BASE = process.env.E2E_URL || 'http://127.0.0.1:8080';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });

// First hydrate at root
await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
console.log('Root hydrated');

const routes = ['/', '/#/transactions', '/#/accounts', '/#/goals', '/#/budgets', '/#/bills'];

for (const route of routes) {
  await page.goto(BASE + route, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(2500);
  const h1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim() ?? 'no h1');
  const h2s = await page.evaluate(() => Array.from(document.querySelectorAll('h2')).map(e => e.textContent.trim()).slice(0, 3));
  const allBtns = await page.evaluate(() => Array.from(document.querySelectorAll('button')).map(b => b.textContent.trim()).filter(t => t.length > 1 && t.length < 40).slice(0, 8));
  console.log(`\nRoute ${route}:`);
  console.log(`  H1: ${h1}`);
  console.log(`  H2s: ${JSON.stringify(h2s)}`);
  console.log(`  Buttons: ${JSON.stringify(allBtns)}`);
  await page.screenshot({ path: path.join(__dirname, 'probe-route-' + route.replace(/[#\/]/g, '-').replace(/^-/, '') + '.png') });
}

await browser.close();
console.log('\nDone');
