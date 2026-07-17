// report_scope_local_verify.mjs — verifies the report-scope split (parity-scan
// defect: "report scope leaks globally"):
//   1. A scope chosen in /reports narrows the report's own figures.
//   2. It does NOT raise the global scope banner or change other pages.
//   3. It survives a reload (report-local persistence).
//   4. The top-bar "Viewing as" lens still works globally (banner everywhere).
// Usage: node e2e/report_scope_local_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1400); };
const banner = async () => (await page.locator('[data-testid="scope-banner"]').count()) ? await page.locator('[data-testid="scope-banner"]').innerText() : "";

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1500);

// 1. Narrow the report to a single member via the Scope panel.
await nav("/reports");
const reportTextBefore = (await page.locator("main").innerText()).slice(0, 2000);
await page.locator('button:has-text("Scope")').first().click();
await page.waitForTimeout(700);
const chip = page.locator(".scope-chip", { hasText: "Marcus Hartley" }).first();
await chip.click();
await page.waitForTimeout(1200);
check("scope chip arms (aria-pressed)", (await chip.getAttribute("aria-pressed")) === "true");
const reportTextAfter = (await page.locator("main").innerText()).slice(0, 2000);
check("report figures change under the local scope", reportTextAfter !== reportTextBefore);
check("no global banner from a report-local scope", (await banner()) === "", await banner());
await page.screenshot({ path: "e2e/report_scope_local.png" });

// 2. Other pages unaffected.
await nav("/dashboard");
check("dashboard shows no scope banner", (await banner()) === "");
await nav("/accounts");
check("accounts shows no scope banner", (await banner()) === "");

// 3. Reload persistence of the report-local scope.
await page.goto(BASE + "/reports", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1800);
const chipReload = page.locator(".scope-chip", { hasText: "Marcus Hartley" }).first();
const panelOpen = await chipReload.count();
check("scope panel reopens for a persisted report scope", panelOpen > 0);
if (panelOpen) {
  check("report scope persisted across reload", (await chipReload.getAttribute("aria-pressed")) === "true");
  // Clear it (leave the app clean for other suites).
  await page.locator('button:has-text("Clear")').first().click().catch(() => {});
  await page.waitForTimeout(800);
}

// 4. The top-bar lens is still global: pick a member in "Viewing as".
const sw = page.locator('[data-testid="member-switcher"]');
if (await sw.count()) {
  const opts = await sw.locator("option").all();
  let mVal = "";
  for (const o of opts) { const t = await o.innerText(); const v = await o.getAttribute("value"); if (v && /Marcus/.test(t)) { mVal = v; break; } }
  await sw.selectOption(mVal);
  await page.waitForTimeout(1000);
  await nav("/dashboard");
  const b = await banner();
  check("'Viewing as' lens raises the global banner on other pages", /Marcus/.test(b), b);
  // Restore Everyone.
  await sw.selectOption("");
  await page.waitForTimeout(800);
} else {
  check("member switcher present", false);
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
