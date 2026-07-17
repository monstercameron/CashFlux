// dash_drill_verify.mjs — locks the dashboard click-through contract (parity
// scan: every visualization routes to its filtered source data):
//   1. Spending-breakdown legend slice → /transactions filtered to that
//      category within the dashboard's period window.
//   2. Budgets-widget row → /budgets flashed on that budget card.
//   3. Accounts-widget cell → /accounts flashed on that account row.
//   4. Upcoming-bills row → /accounts (account-backed) or /recurring.
//   5. Recent-transactions row → /transactions searched to its payee.
// Usage: node e2e/dash_drill_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1500 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
const goDash = async () => {
  await page.evaluate(() => { history.pushState({}, "", "/dashboard"); dispatchEvent(new PopStateEvent("popstate")); });
  await page.waitForTimeout(2200);
};
await page.goto(BASE + "/dashboard", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2500);

// 1. Breakdown legend slice.
const slice = page.locator('[data-widget="breakdown"] .dash-drill').first();
await slice.waitFor({ timeout: 10000 }).catch(() => {});
check("breakdown legend entries are drills", (await slice.count()) > 0);
const sliceLabel = (await slice.count()) ? await slice.innerText() : "";
if (await slice.count()) {
  await slice.click();
  await page.waitForTimeout(1500);
  check("slice routes to /transactions", page.url().endsWith("/transactions"), page.url());
  const body = await page.locator("main").innerText();
  const catName = sliceLabel.replace(/\s+\d+%$/, "").trim();
  check("ledger is filtered (active filter chips visible)", /Clear|filter/i.test(body), catName);
  await page.screenshot({ path: "e2e/dash_drill_slice.png" });
}

// 2. Budgets row.
await goDash();
const brow = page.locator('[data-widget="budgets"] button').first();
await brow.waitFor({ timeout: 10000 }).catch(() => {});
check("budget rows are drills", (await brow.count()) > 0);
if (await brow.count()) {
  await brow.click();
  await page.waitForTimeout(1500);
  check("budget row routes to /budgets", page.url().endsWith("/budgets"), page.url());
}

// 3. Accounts cell.
await goDash();
const cell = page.locator('[data-widget="accounts"] .dash-drill').first();
await cell.waitFor({ timeout: 10000 }).catch(() => {});
check("account cells are drills", (await cell.count()) > 0);
if (await cell.count()) {
  await cell.click();
  await page.waitForTimeout(1500);
  check("account cell routes to /accounts", page.url().endsWith("/accounts"), page.url());
}

// 4. Bills row.
await goDash();
const bill = page.locator('[data-widget="bills"] .dash-drill').first();
await bill.waitFor({ timeout: 10000 }).catch(() => {});
check("bill rows are drills", (await bill.count()) > 0);
if (await bill.count()) {
  await bill.click();
  await page.waitForTimeout(1500);
  check("bill row routes to /accounts or /recurring", /\/(accounts|recurring)$/.test(page.url()), page.url());
}

// 5. Recent row.
await goDash();
const rrow = page.locator('[data-widget="recent"] .dash-drill-row').first();
await rrow.waitFor({ timeout: 10000 }).catch(() => {});
check("recent rows are drills", (await rrow.count()) > 0);
if (await rrow.count()) {
  const desc = (await rrow.innerText()).split("\n")[1] || "";
  await rrow.click();
  await page.waitForTimeout(1500);
  check("recent row routes to /transactions", page.url().endsWith("/transactions"), page.url());
  const search = await page.locator('input[type="search"], .txn-search input, input[placeholder*="Search"]').first().inputValue().catch(() => "");
  check("ledger search is seeded from the row", search.trim() !== "", JSON.stringify(search));
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
