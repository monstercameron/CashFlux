// fresh_health_verify.mjs — locks the balance-health (freshness) card's repair
// actions (parity scan: connection-health card needs refresh/repair actions):
//   1. Stale chips are clickable and jump to the exact account row on /accounts.
//   2. "Update balances" routes to /accounts.
// Usage: node e2e/fresh_health_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1400 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(BASE + "/dashboard", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2500);

const chip = page.locator('[data-testid^="fresh-chip-"]').first();
check("stale-account chips render as buttons", (await chip.count()) > 0);
let acctID = "";
if (await chip.count()) {
  acctID = (await chip.getAttribute("data-testid")).replace("fresh-chip-", "");
  await chip.scrollIntoViewIfNeeded();
  await chip.click();
  await page.waitForTimeout(1500);
  check("chip routes to /accounts", page.url().endsWith("/accounts"), page.url());
  const row = page.locator(`[data-testid="acct-row-${acctID}"]`);
  check("the exact account row exists to land on", (await row.count()) > 0, acctID);
  await page.screenshot({ path: "e2e/fresh_health_focus.png" });
}

// Back to dashboard for the Update balances action.
await page.evaluate(() => { history.pushState({}, "", "/dashboard"); dispatchEvent(new PopStateEvent("popstate")); });
await page.waitForTimeout(2200);
const upd = page.locator('[data-testid="fresh-update-btn"]');
await upd.waitFor({ timeout: 10000 }).catch(() => {});
check("Update balances action present", (await upd.count()) > 0);
if (await upd.count()) {
  await upd.scrollIntoViewIfNeeded();
  await upd.click();
  await page.waitForTimeout(1200);
  check("Update balances routes to /accounts", page.url().endsWith("/accounts"), page.url());
}
console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
