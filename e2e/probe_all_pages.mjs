// Probe all 5 pages in the loop43 flow to understand form structure
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

const navTo = async (title) => {
  await page.click(`nav[aria-label="Main navigation"] a[title="${title}"]`);
  await page.waitForTimeout(1500);
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
      options: el.tagName === 'SELECT' ? Array.from(el.options).map(o => ({ v: o.value.slice(0,20), t: o.text })).slice(0,10) : undefined,
    }));
  });
};

const probeBtns = async () => {
  return page.evaluate(() => {
    return Array.from(document.querySelectorAll('button')).map(b => ({
      text: b.textContent.trim().slice(0,50),
      type: b.type,
    })).filter(b => b.text.length > 0);
  });
};

// Hydrate
await page.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
console.log('Hydrated\n');

// ─── TRANSACTIONS ────────────────────────────────────────────────────────────
await navTo('Transactions');
const h1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('=== TRANSACTIONS (h1:', h1, ')');
await page.screenshot({ path: path.join(__dirname, 'probe-transactions-page.png') });
const txnBtns = await probeBtns();
const txnNewBtn = txnBtns.find(b => /new trans/i.test(b.text));
console.log('New transaction button:', txnNewBtn);
console.log('All buttons:', txnBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 20));

// Click "New transaction"
if (txnNewBtn) {
  await page.click('button', { hasText: 'New transaction' });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(__dirname, 'probe-transactions-form.png') });
  const fields = await probeFields();
  console.log('Transaction form fields:', JSON.stringify(fields, null, 2));
  const formBtns = await probeBtns();
  console.log('Form buttons:', formBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0,15));
}

// ─── ACCOUNTS ────────────────────────────────────────────────────────────────
await navTo('Accounts');
const acctsH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('\n=== ACCOUNTS (h1:', acctsH1, ')');
await page.screenshot({ path: path.join(__dirname, 'probe-accounts-page.png') });
const acctBtns = await probeBtns();
console.log('All buttons:', acctBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 25));
// Look for transfer button
const transferBtn = acctBtns.find(b => /transfer/i.test(b.text));
console.log('Transfer button:', transferBtn);

// ─── GOALS ───────────────────────────────────────────────────────────────────
await navTo('Goals');
const goalsH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('\n=== GOALS (h1:', goalsH1, ')');
await page.screenshot({ path: path.join(__dirname, 'probe-goals-page.png') });
const goalBtns = await probeBtns();
console.log('All buttons:', goalBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 25));
const bodyTxt = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 500));
console.log('Body snippet:', bodyTxt);

// ─── BUDGETS ─────────────────────────────────────────────────────────────────
await navTo('Budgets');
const budgH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('\n=== BUDGETS (h1:', budgH1, ')');
await page.screenshot({ path: path.join(__dirname, 'probe-budgets-page.png') });
const budgBtns = await probeBtns();
console.log('All buttons:', budgBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 25));
const budgBody = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 600));
console.log('Body snippet:', budgBody);

// ─── BILLS ───────────────────────────────────────────────────────────────────
await navTo('Bills');
const billsH1 = await page.evaluate(() => document.querySelector('h1')?.textContent?.trim());
console.log('\n=== BILLS (h1:', billsH1, ')');
await page.screenshot({ path: path.join(__dirname, 'probe-bills-page.png') });
const billBtns = await probeBtns();
console.log('All buttons:', billBtns.filter(b => b.text.length > 2).map(b => b.text).slice(0, 30));
const billsBody = await page.evaluate(() => document.body.textContent.replace(/\s+/g, ' ').slice(0, 600));
console.log('Body snippet:', billsBody);

await browser.close();
console.log('\nProbe done');
