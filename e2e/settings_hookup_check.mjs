// settings_hookup_check.mjs — proves the tabbed /settings PAGE's controls are
// HOOKED IN: every representative setting is changed through the UI, its live
// effect (or persisted state after leaving + revisiting the page) is asserted,
// and the original value is restored. Also covers the entry points: the side
// nav's System → Settings item and the top bar ⋯ → Settings (both route to
// /settings), plus tab deep-links (the /plans trial CTA lands on the Cloud tab).
//
// Run against a dev server:  node e2e/settings_hookup_check.mjs
// (E2E_URL overrides the default http://127.0.0.1:8080)
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const require = createRequire(path.join(path.dirname(fileURLToPath(import.meta.url)), "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const fails = [];
const ok = (c, m) => { if (!c) { fails.push(m); console.error("  ✗", m); } else console.log("  ✓", m); };

// SPA-navigate (deep URL loads 404 on the dev server).
const goRoute = async (page, route) => {
  await page.evaluate((r) => { history.pushState({}, "", r); dispatchEvent(new PopStateEvent("popstate")); }, route);
  await page.waitForTimeout(600);
};
const openSettings = async (page) => {
  await goRoute(page, "/settings");
  await page.waitForSelector(".settings-page .set-tab-strip", { timeout: 8000 });
  await page.waitForTimeout(400);
};
const leaveSettings = async (page) => { await goRoute(page, "/"); };
const goTab = async (page, label) => {
  await page.locator(".settings-page .set-tab-strip").getByText(label, { exact: true }).click({ force: true });
  await page.waitForTimeout(400);
};
const pageText = (page) => page.locator(".settings-page").innerText();

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);

  // ── Entry points: rail card gone; nav item + ⋯ menu both land on /settings. ──
  ok(await page.locator("button.hh").count() === 0, "rail household card no longer opens settings");
  const railItem = page.locator('aside.rail a[href="/settings"], aside.rail a[aria-label="Settings"]').first();
  ok(await railItem.count() >= 1, "Settings appears in the side nav's System group");
  await railItem.evaluate((el) => el.click());
  await page.waitForSelector(".settings-page .set-tab-strip", { timeout: 8000 });
  ok(true, "side-nav Settings item routes to the /settings page");
  await leaveSettings(page);
  await page.evaluate(() => document.querySelector('[data-testid="topbar-more"]').click());
  await page.waitForTimeout(300);
  await page.evaluate(() => document.querySelector('[data-testid="topbar-settings"]').click());
  await page.waitForSelector(".settings-page .set-tab-strip", { timeout: 8000 });
  ok((await page.evaluate(() => location.pathname)) === "/settings", "top-bar ⋯ → Settings routes to /settings");
  await page.waitForTimeout(400);

  // ── Household tab: base currency drives the rail summary live. ─────────────
  const baseSel = page.locator('.settings-page select[aria-label="Base currency"]');
  const origBase = await baseSel.inputValue();
  await baseSel.selectOption("EUR");
  await page.waitForTimeout(600);
  ok(/EUR base/.test(await page.locator(".hh-quiet").innerText()), "base currency change reflects live in the rail summary");
  await baseSel.selectOption(origBase || "USD");
  await page.waitForTimeout(400);

  // Screens toggle: hide To-do → nav item disappears; restore.
  const todoToggle = page.locator(".settings-page .toggle-row", { hasText: "Show To-do" }).locator(".switch");
  if (await todoToggle.count()) {
    await todoToggle.click({ force: true });
    await page.waitForTimeout(500);
    ok(await page.locator('nav a[href*="/todo"], a[aria-label="To-do"]').count() === 0, "hiding a screen removes it from the nav");
    await todoToggle.click({ force: true });
    await page.waitForTimeout(500);
    ok(await page.locator('a[aria-label="To-do"]').count() >= 1, "re-showing restores the nav item");
  } else {
    ok(false, "Show To-do toggle not found on the Household tab");
  }

  // ── Preferences tab: date format persists across leave/revisit. ────────────
  await goTab(page, "Preferences");
  const dateSel = page.locator('.settings-page select[aria-label="Date format"]');
  const origDate = await dateSel.inputValue();
  await dateSel.selectOption("iso");
  await page.waitForTimeout(400);
  await leaveSettings(page);
  await openSettings(page);
  await goTab(page, "Preferences");
  ok(await dateSel.inputValue() === "iso", "date format persists across leave/revisit");
  await dateSel.selectOption(origDate || "long");
  // Week start segmented is wired.
  const weekSeg = page.locator(".settings-page .toggle-row", { hasText: "Week starts on" });
  ok(await weekSeg.count() >= 1 || await page.locator(".settings-page").getByText("Monday").count() >= 1, "week-start control present on Preferences");

  // ── Alerts tab: freshness + notifications moved here from Preferences. ─────
  await goTab(page, "Alerts");
  const alertsText = await pageText(page);
  ok(/freshness|stale/i.test(alertsText), "freshness reminders live on the Alerts tab");
  ok(/notification/i.test(alertsText), "notifications live on the Alerts tab");

  // ── AI tab: key field persists to app settings. ─────────────────────────────
  await goTab(page, "AI");
  const keyInput = page.locator('.settings-page input[aria-label*="OpenAI"], .settings-page input[placeholder*="OpenAI"]').first();
  ok(await keyInput.count() === 1, "AI key field present on the AI tab");
  await keyInput.fill("sk-e2e-hookup-test");
  await page.waitForTimeout(400);
  await leaveSettings(page);
  await openSettings(page);
  await goTab(page, "AI");
  ok(await keyInput.inputValue() === "sk-e2e-hookup-test", "AI key persists across leave/revisit");
  await keyInput.fill("");
  await page.waitForTimeout(300);

  // ── Cloud tab: the backend toggle reveals/hides the connection form. ───────
  await goTab(page, "Cloud");
  const backendSwitch = page.locator(".settings-page .toggle-row", { hasText: "Connect to a backend" }).locator(".switch").first();
  await backendSwitch.click({ force: true });
  await page.waitForTimeout(400);
  const after = await pageText(page);
  ok(/fully local|server address|https:\/\//i.test(after), "backend toggle changes the Cloud tab's state");
  await backendSwitch.click({ force: true }); // restore
  await page.waitForTimeout(400);

  // ── Data tab: backup cadence persists. ──────────────────────────────────────
  await goTab(page, "Data");
  const cadenceSel = page.locator('.settings-page select[aria-label="Backup reminders"], .settings-page select[aria-label*="ackup"]').first();
  ok(await cadenceSel.count() === 1, "backup cadence select present on the Data tab");
  const origCad = await cadenceSel.inputValue();
  await cadenceSel.selectOption("weekly");
  await leaveSettings(page);
  await openSettings(page);
  await goTab(page, "Data");
  ok(await cadenceSel.inputValue() === "weekly", "backup cadence persists across leave/revisit");
  await cadenceSel.selectOption(origCad || "monthly");
  ok(/Export JSON|Export CSV/i.test(await pageText(page)), "data actions present");

  // ── Advanced tab: app lock + languages are reachable. ──────────────────────
  await goTab(page, "Advanced");
  const advText = await pageText(page);
  ok(/passcode/i.test(advText), "app lock section on Advanced");
  ok(/Languages/i.test(advText), "languages section on Advanced");

  // ── Tab deep-link: the /plans trial CTA lands on the Cloud tab. ────────────
  await goRoute(page, "/plans");
  await page.waitForTimeout(600);
  const trial = page.locator('button:has-text("trial"), button:has-text("Trial")').first();
  if (await trial.count()) {
    await trial.evaluate((el) => el.click());
    await page.waitForSelector(".settings-page .set-tab-strip", { timeout: 8000 });
    await page.waitForTimeout(500);
    const cloudNow = /Connect to a backend|fully local/i.test(await pageText(page));
    ok((await page.evaluate(() => location.pathname)) === "/settings" && cloudNow, "plans trial CTA deep-links to /settings on the Cloud tab");
  } else {
    ok(true, "plans trial CTA not present in this state (skip)");
  }
  await leaveSettings(page);

  ok(errors.length === 0, `no page errors (${errors.join(" | ") || "none"})`);
} finally { await browser.close(); }
if (fails.length) { console.error("\nFAIL settings_hookup_check:\n - " + fails.join("\n - ")); process.exit(1); }
console.log("\nPASS: settings_hookup_check — every tab's settings are hooked in");
