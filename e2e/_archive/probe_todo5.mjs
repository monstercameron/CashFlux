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
  await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);

  await page.locator('nav a[title="To-do"]').first().click();
  await page.waitForTimeout(3000);

  // Get full app HTML
  const appHtml = await page.evaluate(() => document.getElementById('app')?.innerHTML || 'no app');
  // Look for task-add specifically
  const taskAddEl = await page.evaluate(() => {
    const el = document.getElementById('task-add');
    return el ? { found: true, tagName: el.tagName, outerHtml: el.outerHTML.slice(0,200) } : { found: false };
  });
  console.log('task-add element:', JSON.stringify(taskAddEl));

  // Print app HTML around the content area
  // Look for .todo, .tasks, .task-list
  const contentSelectors = ['.todo', '.tasks', '.task-list', '.todo-screen', '.screen', 'article', '.content', '.card', '[class*="todo"]', '[class*="task"]'];
  for (const sel of contentSelectors) {
    const count = await page.locator(sel).count();
    if (count > 0) {
      const html = await page.locator(sel).first().innerHTML();
      console.log(`\n=== ${sel} (${count} found) ===`);
      console.log(html.slice(0, 500));
    }
  }
} finally {
  await browser.close();
}
