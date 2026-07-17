// tax_workflow_verify.mjs — locks the named tax-deductible workflow (parity
// scan item 23): mark a category deductible → the Annual Review's "Deductible
// totals" section appears for the year window with a headline total, drillable
// supporting-transaction rows, and a clean CSV export — no formulas required.
// Usage: node e2e/tax_workflow_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1300 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1600); };
await page.goto(BASE + "/categories", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2200);

// 1. Mark a category deductible (edit form checkbox). Use "Groceries"? No —
// pick a small one: open the first category's edit and tick the box.
const editBtn = page.locator('button:has-text("Edit")').first();
await editBtn.click();
await page.waitForTimeout(900);
const dedBox = page.locator('[data-testid^="cat-deductible-"]').first();
check("deductible checkbox exists on the category form", (await dedBox.count()) > 0);
const already = await dedBox.isChecked().catch(() => false);
if (!already) await dedBox.check();
await page.locator('button:has-text("Save")').first().click();
await page.waitForTimeout(1200);

// 2. The Annual Review shows the named Deductible totals section for the year.
await nav("/reports");
await page.waitForTimeout(2500);
const section = page.locator('[data-testid="deductible-section"]');
await section.scrollIntoViewIfNeeded().catch(() => {});
check("Deductible totals section renders", (await section.count()) === 1);
if (await section.count()) {
  const text = (await section.innerText()).replace(/\s+/g, " ");
  check("headline total present", /\$[\d,.]+/.test(text), text.slice(0, 120));
  check("CSV export present", (await section.locator('[data-testid="deductible-download-csv"]').count()) > 0);
  const row = section.locator('[data-testid^="deductible-row-"]').first();
  check("rows are supporting-transaction drills", (await row.count()) > 0);
  await page.screenshot({ path: "e2e/tax_workflow_section.png" });
  if (await row.count()) {
    await row.click();
    await page.waitForTimeout(1800);
    check("drill routes to the filtered ledger", page.url().endsWith("/transactions"), page.url());
    const body = await page.locator("main").innerText();
    check("ledger carries the year window filter", /From 20\d\d-\d\d-\d\d/.test(body));
  }
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
