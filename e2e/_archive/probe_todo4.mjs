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
  // Navigate to root first (wasm boots here)
  await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
  // Wait for nav to appear
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);

  // Click To-do in the nav
  await page.locator('nav a[title="To-do"]').first().click();
  await page.waitForTimeout(2000);

  const url = page.url();
  console.log('current url:', url);

  const inputs = await page.evaluate(() => [...document.querySelectorAll('input')].map(i => ({ id: i.id, type: i.type, placeholder: i.placeholder, class: i.className.slice(0,60) })));
  console.log('inputs:', JSON.stringify(inputs, null, 2));

  const forms = await page.evaluate(() => [...document.querySelectorAll('form')].map(f => ({ id: f.id, class: f.className.slice(0,80) })));
  console.log('forms:', JSON.stringify(forms, null, 2));

  const mainHtml = await page.evaluate(() => document.querySelector('main, #main')?.innerHTML?.slice(0, 4000) || 'not found');
  console.log('main HTML:', mainHtml);

  await page.screenshot({ path: path.join(__dirname, 'screenshots', 'todo_probe4.png'), fullPage: true });
  console.log('screenshot taken');
} finally {
  await browser.close();
}
