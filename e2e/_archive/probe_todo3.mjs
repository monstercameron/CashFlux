import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import path from 'path';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');

const BASE = process.env.E2E_URL || 'http://127.0.0.1:8099';
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE + '/todo', { waitUntil: 'domcontentloaded' });
  // Wait for wasm to init — look for main content appearing
  await page.waitForSelector('main, #main, .screen, [role="main"]', { timeout: 60000 });
  // Poll for inputs to appear (wasm renders asynchronously)
  await page.waitForFunction(() => document.querySelectorAll('input').length > 0, { timeout: 90000 });
  await page.waitForTimeout(1000);

  const inputs = await page.evaluate(() => [...document.querySelectorAll('input')].map(i => ({ id: i.id, type: i.type, placeholder: i.placeholder, class: i.className.slice(0,60) })));
  console.log('inputs:', JSON.stringify(inputs, null, 2));

  const mainHtml = await page.evaluate(() => document.querySelector('main, #main')?.innerHTML?.slice(0, 5000) || 'not found');
  console.log('main HTML:', mainHtml);
} finally {
  await browser.close();
}
