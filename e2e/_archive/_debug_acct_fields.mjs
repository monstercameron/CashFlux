import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import path from 'path';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');
const BASE = 'http://127.0.0.1:8099';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
try {
  await page.goto(BASE + '/accounts', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.locator('.add-btn').click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForTimeout(400);
  await page.waitForSelector('.cf-adv-toggle', { timeout: 10000 });
  await page.locator('.cf-adv-toggle').first().click();
  await page.waitForTimeout(300);

  // Check all number inputs and their attributes
  const inputs = await page.locator('input[type="number"]').evaluateAll(els =>
    els.map(el => ({ ph: el.placeholder, min: el.min, max: el.max, step: el.step, aria: el.getAttribute('aria-label') }))
  );
  console.log('Number inputs:', JSON.stringify(inputs, null, 2));

  // Check selects
  const selects = await page.locator('select').evaluateAll(els =>
    els.map(el => ({ aria: el.getAttribute('aria-label'), opts: Array.from(el.options).slice(0,5).map(o=>o.text) }))
  );
  console.log('Selects:', JSON.stringify(selects, null, 2));
} finally {
  await browser.close();
}
