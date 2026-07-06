import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import path from 'path';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');
const BASE = 'http://127.0.0.1:8099';
const NAME = 'DEBUG-GOAL-' + Date.now();
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
try {
  await page.goto(BASE + '/goals', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.locator('.add-btn').click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator('#goal-add').fill(NAME);
  await dialog.locator('input[type="number"][aria-required="true"]').fill('1000');
  await dialog.locator('input[type="number"]').nth(1).fill('100');
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1000);

  // Check all elements that contain NAME
  const found = await page.evaluate((name) => {
    const all = Array.from(document.querySelectorAll('*'));
    const matching = all.filter(el => el.childNodes.length > 0 && el.textContent.includes(name) && el.textContent.length < 200);
    return matching.slice(0, 5).map(el => ({ tag: el.tagName, classes: el.className, text: el.textContent.trim().substring(0, 80) }));
  }, NAME);
  console.log('Elements containing NAME:', JSON.stringify(found, null, 2));

  // Also check localStorage
  const ls = await page.evaluate(() => {
    const d = JSON.parse(localStorage.getItem('cashflux:dataset') || '{}');
    return (d.goals || []).slice(-3).map(g => g.name);
  });
  console.log('Recent goals in localStorage:', ls);
} finally {
  await browser.close();
}
