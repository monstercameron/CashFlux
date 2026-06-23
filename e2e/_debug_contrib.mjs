import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import path from 'path';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');
const BASE = 'http://127.0.0.1:8099';
const browser = await chromium.launch({ headless: false }); // visible for debug
const page = await browser.newPage();
try {
  await page.goto(BASE + '/goals', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.waitForTimeout(2000);

  // Add a goal via modal
  await page.locator('.add-btn').click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator('#goal-add').fill('DEBUG-CONTRIB-GOAL');
  await dialog.locator('input[type="number"][aria-required="true"]').fill('500');
  await dialog.locator('input[type="number"]').nth(1).fill('475');
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);
  // Soft nav
  await page.evaluate(() => { window.history.pushState({}, '', '/'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.history.pushState({}, '', '/goals'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(800);

  // Find contribute button
  const allBtns = await page.$$('button');
  let contribBtn = null;
  for (const btn of allBtns) {
    const info = await btn.evaluate((el, name) => {
      const txt = el.textContent?.trim() ?? '';
      const row = el.closest('li, tr, [class*="goal"], [class*="row"], article, section') ?? el.parentElement;
      const rowTxt = row ? row.textContent ?? '' : '';
      return { txt, inRow: rowTxt.includes(name) };
    }, 'DEBUG-CONTRIB-GOAL');
    if (/^contribute$/i.test(info.txt) && info.inRow) { contribBtn = btn; break; }
  }
  if (!contribBtn) { console.log('No Contribute button found'); }
  else {
    await contribBtn.click();
    await page.waitForTimeout(500);
    const inputs = await page.locator('input').evaluateAll(els => els.map(el => ({ id: el.id, ph: el.placeholder, val: el.value })));
    console.log('Inputs after Contribute click:', JSON.stringify(inputs.filter(i => i.ph || i.id), null, 2));
  }
  await page.waitForTimeout(3000);
} finally {
  await browser.close();
}
