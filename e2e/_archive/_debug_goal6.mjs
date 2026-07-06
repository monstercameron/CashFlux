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
const softNav = async (route) => {
  await page.evaluate((r) => { window.history.pushState({}, '', r); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); }, route);
  await page.waitForTimeout(800);
};
try {
  await page.goto(BASE + '/goals', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.waitForTimeout(1500);

  await page.locator('.add-btn').click();
  await page.waitForTimeout(400);
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForTimeout(600);
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator('#goal-add').fill(NAME);
  await dialog.locator('input[type="number"][aria-required="true"]').fill('1000');
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1000);

  // Soft navigate away and back
  await softNav('/');
  await softNav('/goals');

  const found = await page.evaluate((name) => {
    const all = Array.from(document.querySelectorAll('*'));
    return all.some(el => el.textContent.includes(name) && el.textContent.length < 300);
  }, NAME);
  console.log('Goal found after soft nav:', found);
} finally {
  await browser.close();
}
