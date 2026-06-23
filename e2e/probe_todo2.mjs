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
  await page.waitForTimeout(5000); // give wasm time to render

  const inputs = await page.evaluate(() => [...document.querySelectorAll('input')].map(i => ({ id: i.id, type: i.type, class: i.className.slice(0,60) })));
  console.log('inputs:', JSON.stringify(inputs, null, 2));

  const forms = await page.evaluate(() => [...document.querySelectorAll('form')].map(f => ({ id: f.id, class: f.className.slice(0,80) })));
  console.log('forms:', JSON.stringify(forms, null, 2));

  const textareas = await page.evaluate(() => [...document.querySelectorAll('textarea')].map(t => ({ id: t.id, placeholder: t.placeholder, class: t.className.slice(0,60) })));
  console.log('textareas:', JSON.stringify(textareas, null, 2));

  const mainHtml = await page.evaluate(() => document.querySelector('main, #main, .screen')?.innerHTML?.slice(0, 3000) || 'not found');
  console.log('main HTML:', mainHtml);
} finally {
  await browser.close();
}
