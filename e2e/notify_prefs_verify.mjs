// notify_prefs_verify.mjs — locks per-event notification preferences (parity
// scan item 24): Settings → Notifications exposes one enable toggle per alert
// type (bill due, budget, stale balance, digest, backup, large txn, low
// balance, paycheck, …), tunable thresholds where they apply, and a separate
// browser-notification channel toggle. Toggles persist across a reload.
// Usage: node e2e/notify_prefs_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 1300 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(BASE + "/settings", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2200);

// Open the notifications tab if tabbed.
const tab = page.locator('button:has-text("Alerts"), [role="tab"]:has-text("Alerts")').first();
if (await tab.count()) { await tab.click(); await page.waitForTimeout(900); }

const zone = page.locator('[data-testid="settings-manage-alerts"]');
await zone.scrollIntoViewIfNeeded().catch(() => {});
check("Manage-alerts zone renders", (await zone.count()) === 1);
const toggles = zone.locator('input[type="checkbox"], [role="switch"], button[aria-pressed]');
const nToggles = await toggles.count();
check("one control per alert type (5+)", nToggles >= 5, `${nToggles} controls`);
const zoneText = (await zone.innerText()).replace(/\s+/g, " ");
check("thresholds are tunable (days / $)", (await zone.locator("input[type=number], input[inputmode=numeric], input[inputmode=decimal]").count()) > 0, zoneText.slice(0, 120));
const notifZone = page.locator('[data-testid="settings-notifications"]');
check("browser-notification channel toggle present", /browser/i.test(await notifZone.innerText()));
await page.screenshot({ path: "e2e/notify_prefs_settings.png" });

// Persistence: flip the FIRST alert toggle, reload, expect it kept.
const first = toggles.first();
const before = await first.isChecked().catch(async () => (await first.getAttribute("aria-pressed")) === "true");
await first.click();
await page.waitForTimeout(1200);
await page.reload({ waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2000);
const tab2 = page.locator('button:has-text("Alerts"), [role="tab"]:has-text("Alerts")').first();
if (await tab2.count()) { await tab2.click(); await page.waitForTimeout(900); }
const first2 = page.locator('[data-testid="settings-manage-alerts"]').locator('input[type="checkbox"], [role="switch"], button[aria-pressed]').first();
const after = await first2.isChecked().catch(async () => (await first2.getAttribute("aria-pressed")) === "true");
check("per-event toggle persists across reload", after === !before, `${before} → ${after}`);
// Restore.
await first2.click();
await page.waitForTimeout(800);

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
