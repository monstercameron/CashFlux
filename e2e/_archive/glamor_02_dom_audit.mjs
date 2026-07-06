// DOM audit for G2 Transactions GLAMOR review
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
const page = await ctx.newPage();
await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
await page.waitForSelector('nav a[title]', { timeout: 60000 });
await page.waitForTimeout(1500);

try {
  await page.locator('nav a[title="Transactions"]').first().click();
} catch (_) {
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
}
await page.waitForTimeout(1500);

const detail = await page.evaluate(() => {
  const rows = Array.from(document.querySelectorAll('tr[data-id]'));
  const rowHeights = rows.slice(0, 3).map(r => r.getBoundingClientRect().height);

  const ths = Array.from(document.querySelectorAll('th'));
  const colWidths = ths.map(th => ({ label: th.innerText.trim(), width: Math.round(th.getBoundingClientRect().width) }));

  const amountCells = Array.from(document.querySelectorAll('.td-amount'));
  const amountAlign = amountCells.slice(0, 3).map(el => window.getComputedStyle(el).textAlign);
  const amountFont = amountCells[1] ? window.getComputedStyle(amountCells[1]).fontVariantNumeric : null;
  const amountFontFamily = amountCells[1] ? window.getComputedStyle(amountCells[1]).fontFamily : null;

  const actionCells = Array.from(document.querySelectorAll('.td-actions'));
  const actionTexts = actionCells.slice(0, 2).map(el => el.innerText.trim().replace(/\n+/g, ' | '));

  const firstIncome = Array.from(document.querySelectorAll('.td-amount')).find(el => !el.innerText.startsWith('(') && el.innerText !== 'Amount');
  const firstExpense = Array.from(document.querySelectorAll('.td-amount')).find(el => el.innerText.startsWith('('));
  const incomeColor = firstIncome ? window.getComputedStyle(firstIncome).color : null;
  const expenseColor = firstExpense ? window.getComputedStyle(firstExpense).color : null;

  const paginationEl = document.querySelector('.pagination, [class*="pagination"]');
  const paginationText = paginationEl ? paginationEl.innerText.trim() : null;

  const mutedEls = Array.from(document.querySelectorAll('.muted'));
  const summaryTexts = mutedEls.map(el => el.innerText.trim()).filter(Boolean);

  const clearedCells = Array.from(document.querySelectorAll('.td-cleared')).slice(0, 3).map(el => el.innerText.trim());

  const row0bg = rows[0] ? window.getComputedStyle(rows[0]).backgroundColor : null;
  const row1bg = rows[1] ? window.getComputedStyle(rows[1]).backgroundColor : null;
  const row2bg = rows[2] ? window.getComputedStyle(rows[2]).backgroundColor : null;

  const tagCells = Array.from(document.querySelectorAll('.td-tags')).slice(0, 5).map(el => el.innerText.trim());

  // Check table has thead/tbody
  const tableHasThead = !!document.querySelector('table thead');
  const tableHasTbody = !!document.querySelector('table tbody');

  // Filter toolbar structure
  const toolbarBtns = Array.from(document.querySelectorAll('.toolbar button, .filter-toolbar button, [class*="toolbar"] button')).map(b => b.innerText.trim());

  // Card title
  const cardTitle = document.querySelector('.card-title, h2')?.innerText?.trim();

  // Page size selector
  const pageSizeSel = document.querySelector('select[aria-label*="page"], select[aria-label*="Page"], select[aria-label*="per page"]');
  const pageSizeOptions = pageSizeSel ? Array.from(pageSizeSel.options).map(o => o.value) : [];

  // Count visible buttons in actions col
  const actionBtnCounts = actionCells.slice(0, 3).map(el => el.querySelectorAll('button').length);

  // Check if amounts have tabular-nums via class or style
  const amountEl = amountCells[1];
  const amountClass = amountEl ? amountEl.className : null;

  return {
    rowHeights, colWidths, amountAlign, amountFont, amountFontFamily, amountClass,
    actionTexts, actionBtnCounts, incomeColor, expenseColor,
    paginationText, summaryTexts, clearedCells,
    row0bg, row1bg, row2bg, tagCells,
    tableHasThead, tableHasTbody, toolbarBtns, cardTitle, pageSizeOptions
  };
});

console.log(JSON.stringify(detail, null, 2));
fs.writeFileSync(
  path.join(__dirname, "screenshots", "glamor_02_dom_audit.json"),
  JSON.stringify(detail, null, 2)
);
await browser.close();
