// reports_range_verify.mjs — locks the reports range/click-through/partial and
// saved-view contracts (parity scan items 19-22):
//   1. The review window walks with the period control (arbitrary end month).
//   2. A month row drills to the ledger filtered to exactly that month.
//   3. The in-progress month is labeled at the masthead AND in the table.
//   4. Scope saved views: save-current-as and re-apply round-trips.
// Usage: node e2e/reports_range_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1400 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
page.on("dialog", (d) => d.accept("QA saved view"));
await page.goto(BASE + "/reports", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2500);

// 1. Window follows the period control.
const title0 = await page.locator(".rpta-title").innerText();
await page.locator('[data-testid="period-prev"], button[aria-label*="Prev"], button[title*="Prev"]').first().click();
await page.waitForTimeout(1800);
const title1 = await page.locator(".rpta-title").innerText();
check("review window walks with the period control", title0 !== title1, `${title0} → ${title1}`);
await page.locator('[data-testid="period-next"], button[aria-label*="Next"], button[title*="Next"]').first().click();
await page.waitForTimeout(1800);

// 2. Partial-month labels.
check("masthead labels the in-progress month", (await page.locator('[data-testid="rpta-partial-chip"]').count()) === 1,
  await page.locator('[data-testid="rpta-partial-chip"]').innerText().catch(() => ""));
check("year-in-motion tags the in-progress month row", (await page.locator('[data-testid="rpta-inprogress"]').count()) >= 1);
await page.screenshot({ path: "e2e/reports_partial_chip.png", clip: { x: 240, y: 0, width: 1200, height: 500 } });

// 3. Month-row drill → ledger filtered to that month.
const drill = page.locator(".rpta-month-drill").first();
check("month rows are drills", (await drill.count()) > 0);
const monthLabel = (await drill.count()) ? await drill.innerText() : "";
if (await drill.count()) {
  await drill.scrollIntoViewIfNeeded();
  await drill.click();
  await page.waitForTimeout(1800);
  check("month drill routes to /transactions", page.url().endsWith("/transactions"), page.url());
  const body = await page.locator("main").innerText();
  check("ledger carries the month's From/To filter", /From 20\d\d-\d\d-01/.test(body) && /To 20\d\d-\d\d-\d\d/.test(body), monthLabel);
}

// 4. Saved views round-trip: back on /reports, open Scope, pick a chip, save, clear, re-apply.
await page.evaluate(() => { history.pushState({}, "", "/reports"); dispatchEvent(new PopStateEvent("popstate")); });
await page.waitForTimeout(2000);
await page.locator('button:has-text("Scope")').first().click();
await page.waitForTimeout(700);
await page.locator(".scope-chip", { hasText: "Checking" }).first().click();
await page.waitForTimeout(900);
await page.locator('button:has-text("Save current as")').first().click();
await page.waitForTimeout(900);
// Naming UI: either a prompt (auto-accepted above) or an inline input + confirm.
const nameInput = page.locator('input[placeholder="View name"]').first();
if (await nameInput.count()) {
  await nameInput.fill("QA saved view");
  await page.locator('button:has-text("Save")').nth(1).click().catch(async () => { await page.getByRole("button", { name: "Save", exact: true }).click(); });
  await page.waitForTimeout(900);
}
await page.locator('button:has-text("View all"), button:has-text("Clear")').first().click();
await page.waitForTimeout(700);
const chipOff = await page.locator(".scope-chip", { hasText: "Checking" }).first().getAttribute("aria-pressed");
check("clear resets the scope", chipOff === "false", chipOff);
const sv = page.locator("select").filter({ has: page.locator('option:has-text("QA saved view")') }).first();
check("saved view appears in the picker", (await sv.count()) > 0);
if (await sv.count()) {
  await sv.selectOption({ label: "QA saved view" });
  await page.waitForTimeout(1200);
  const chipOn = await page.locator(".scope-chip", { hasText: "Checking" }).first().getAttribute("aria-pressed");
  check("applying the saved view restores the scope", chipOn === "true", chipOn);
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
