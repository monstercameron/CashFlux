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
  await page.waitForTimeout(2000); // let wasm init fully

  // Check localStorage before adding
  const before = await page.evaluate(() => {
    const d = JSON.parse(localStorage.getItem('cashflux:dataset') || '{}');
    return { goalCount: (d.goals || []).length, keys: Object.keys(d) };
  });
  console.log('Before:', before);

  await page.locator('.add-btn').click();
  await page.waitForTimeout(400);
  const menuItems = await page.locator('[role="menuitem"]').allTextContents();
  console.log('Menu items:', menuItems);

  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForTimeout(600);
  const dialogCount = await page.locator('[role="dialog"]').count();
  console.log('Dialog count after click:', dialogCount);

  if (dialogCount > 0) {
    const dialog = page.locator('[role="dialog"]');
    await dialog.locator('#goal-add').fill(NAME);
    await dialog.locator('input[type="number"][aria-required="true"]').fill('1000');
    await dialog.locator('button[type="submit"]').first().click();
    await page.waitForTimeout(1000);
    const after = await page.evaluate(() => {
      const d = JSON.parse(localStorage.getItem('cashflux:dataset') || '{}');
      return { goalCount: (d.goals || []).length };
    });
    console.log('After submit:', after);
  }

  const pageText = await page.evaluate(() => document.body.innerText.substring(500, 1200));
  console.log('Goals section:', pageText);
} finally {
  await browser.close();
}
