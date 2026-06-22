// Probe accounts, goals, budgets, bills pages
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

// Hydrate
await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
console.log('Hydrated');

const navTo = async (title) => {
  // Use JS click to avoid overlay issues
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link = links.find(l => l.getAttribute('title') === t);
    if (link) link.click();
    else console.warn('Nav link not found:', t);
  }, title);
  await page.waitForTimeout(1500);
};

const probeBtns = async () => {
  return page.evaluate(() => {
    return Array.from(document.querySelectorAll('button')).map(b => ({
      text: b.textContent.trim().slice(0, 50),
      type: b.type,
    })).filter(b => b.text.length > 1);
  });
};

const probeFields = async () => {
  return page.evaluate(() => {
    const inputs = Array.from(document.querySelectorAll('input, select, textarea'));
    return inputs.map(el => ({
      tag: el.tagName,
      type: el.getAttribute('type') ?? '',
      id: el.id,
      ariaLabel: el.getAttribute('aria-label') ?? '',
      placeholder: el.getAttribute('placeholder') ?? '',
      options: el.tagName === 'SELECT' ? Array.from(el.options).map(o => o.text).slice(0,8) : undefined,
    }));
  });
};

// ─── ACCOUNTS ────────────────────────────────────────────────────────────────
await navTo('Accounts');
console.log('\n=== ACCOUNTS');
console.log('URL:', page.url());
const accH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('H1:', accH1);
await page.screenshot({ path: path.join(__dirname, 'probe2-accounts.png') });
const accBtns = await probeBtns();
console.log('Buttons:', accBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 30));
const accBody = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 800));
console.log('Body:', accBody);

// ─── GOALS ───────────────────────────────────────────────────────────────────
await navTo('Goals');
console.log('\n=== GOALS');
const goH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('H1:', goH1);
await page.screenshot({ path: path.join(__dirname, 'probe2-goals.png') });
const goBtns = await probeBtns();
console.log('Buttons:', goBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 30));
const goBody = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 800));
console.log('Body:', goBody);

// ─── BUDGETS ─────────────────────────────────────────────────────────────────
await navTo('Budgets');
console.log('\n=== BUDGETS');
const buH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('H1:', buH1);
await page.screenshot({ path: path.join(__dirname, 'probe2-budgets.png') });
const buBtns = await probeBtns();
console.log('Buttons:', buBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 30));
const buBody = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 800));
console.log('Body:', buBody);

// ─── BILLS ───────────────────────────────────────────────────────────────────
await navTo('Bills');
console.log('\n=== BILLS');
const biH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('H1:', biH1);
await page.screenshot({ path: path.join(__dirname, 'probe2-bills.png') });
const biBtns = await probeBtns();
console.log('Buttons:', biBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 30));
const biBody = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 1000));
console.log('Body:', biBody);

await browser.close();
console.log('\nDone');
