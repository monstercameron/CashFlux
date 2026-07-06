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
  await page.goto(BASE + '/todo', { waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);
  await page.screenshot({ path: path.join(__dirname, 'screenshots', 'todo_probe.png'), fullPage: true });
  const html = await page.evaluate(() => document.body.innerHTML.slice(0, 2000));
  console.log('HTML snippet:', html);
  const inputs = await page.evaluate(() => [...document.querySelectorAll('input')].map(i => ({ id: i.id, type: i.type, class: i.className })));
  console.log('inputs:', JSON.stringify(inputs));
} finally {
  await browser.close();
}
