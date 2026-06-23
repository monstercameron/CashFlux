import { createRequire } from 'module';
import path from 'path';
import { fileURLToPath } from 'url';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');
const BASE = process.env.E2E_URL || 'http://127.0.0.1:8099';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });

// Hydrate at root
await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
console.log('Root hydrated');

// Try clicking the nav links
const navLinks = await page.evaluate(() => {
  return Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')).map(a => ({
    title: a.getAttribute('title'),
    href: a.getAttribute('href'),
    class: a.className.slice(0, 60),
  }));
});
console.log('Nav links:', JSON.stringify(navLinks, null, 2));

// Click Transactions nav link
const txnLink = navLinks.find(l => /trans/i.test(l.title));
console.log('Transactions nav link:', txnLink);
if (txnLink) {
  await page.click(`nav[aria-label="Main navigation"] a[title="${txnLink.title}"]`);
  await page.waitForTimeout(2000);
  const h1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim() ?? 'no h1');
  const url = page.url();
  console.log(`After clicking "${txnLink.title}": H1="${h1}", URL="${url}"`);
  await page.screenshot({ path: path.join(__dirname, 'probe-nav-click-txn.png') });
}

// Get ALL nav link titles
console.log('All nav titles:', navLinks.map(l => l.title));

await browser.close();
