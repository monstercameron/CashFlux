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
  // Boot at root first (like add_modal_check.mjs), then navigate to /goals
  await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.waitForTimeout(2000); // let wasm init fully

  // Now add via modal
  await page.locator('.add-btn').click();
  await page.waitForTimeout(400);
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForTimeout(600);
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator('#goal-add').fill(NAME);
  await dialog.locator('input[type="number"][aria-required="true"]').fill('1000');
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1000);

  // Navigate to /goals
  await page.evaluate(() => { window.history.pushState({}, '', '/goals'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(1500);

  // Check if goal appears
  const found = await page.evaluate((name) => {
    const all = Array.from(document.querySelectorAll('*'));
    return all.some(el => el.textContent.includes(name) && el.textContent.length < 300);
  }, NAME);
  console.log('Goal found on /goals after navigate:', found);

  // Also check page body
  const body = await page.evaluate(() => document.body.innerText.substring(300, 1000));
  console.log('Body:', body);
} finally {
  await browser.close();
}
