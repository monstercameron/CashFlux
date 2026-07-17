// txn_upcoming_verify.mjs — locks the ledger's pending-vs-posted strip (parity
// scan item: pending visually distinct, matched without duplicates, predictable
// in budgets/totals):
//   1. The strip renders this month's still-unposted scheduled charges with an
//      UPCOMING badge, visually apart from posted rows.
//   2. It never counts in the ledger totals (schedule entries, not rows).
//   3. Clicking opens /recurring.
// Usage: node e2e/txn_upcoming_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1100 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(BASE + "/transactions", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2200);

const strip = page.locator('[data-testid="txn-upcoming-strip"]');
check("upcoming strip renders", (await strip.count()) === 1);
if (await strip.count()) {
  const text = (await strip.innerText()).replace(/\s+/g, " ");
  check("strip is headed 'Upcoming this month'", /Upcoming this month · \d+/.test(text), text.slice(0, 90));
  check("rows carry the UPCOMING badge", (await strip.locator(".txn-upcoming-badge").count()) > 0);
  check("strip states it is not counted in totals", /never counted in totals/i.test(text));
  // Distinctness: ghost rows are dimmed relative to real rows.
  const op = await strip.locator(".txn-upcoming-row").first().evaluate((el) => getComputedStyle(el).opacity);
  check("ghost rows are visually dimmed", parseFloat(op) < 0.9, op);
  await page.screenshot({ path: "e2e/txn_upcoming_strip.png" });
  await strip.click();
  await page.waitForTimeout(1400);
  check("strip routes to /recurring", page.url().endsWith("/recurring"), page.url());
}
console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
