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

// First go to root and wait for hydration
await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
console.log('Root hydrated');

// Now navigate to transactions
await page.goto(BASE + '/#/transactions', { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2000);

const h1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim() ?? 'no h1');
console.log('H1:', h1);
const url = page.url();
console.log('URL:', url);

// Probe form fields
const formFields = await page.evaluate(() => {
  const inputs = Array.from(document.querySelectorAll('input, select, textarea'));
  return inputs.map(el => ({
    tag: el.tagName,
    type: el.getAttribute('type') ?? '',
    id: el.id,
    ariaLabel: el.getAttribute('aria-label') ?? '',
    placeholder: el.getAttribute('placeholder') ?? '',
    options: el.tagName === 'SELECT' ? Array.from(el.options).map(o => o.text).slice(0, 5) : undefined,
  }));
});
console.log('Form fields:', JSON.stringify(formFields, null, 2));

// All buttons
const btns = await page.evaluate(() => {
  return Array.from(document.querySelectorAll('button')).map(b => ({ text: b.textContent.trim().slice(0, 30), type: b.type, id: b.id }));
});
console.log('Buttons:', JSON.stringify(btns));

await page.screenshot({ path: path.join(__dirname, 'probe-transactions.png') });
console.log('Screenshot saved');
await browser.close();
