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
  const dialogCount = await page.locator('[role="dialog"]').count();
  console.log('Dialog still open:', dialogCount);
  const budgetCount = await page.locator('.budget', { hasText: NAME }).count();
  console.log('Budget rows found:', budgetCount);
  const url = page.url();
  console.log('Current URL:', url);
  const bodySnippet = await page.evaluate(() => document.body.innerText.substring(0, 800));
  console.log('Body:', bodySnippet);
} finally {
  await browser.close();
}
