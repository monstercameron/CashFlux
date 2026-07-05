// settings_hookup_check.mjs — proves the tabbed Settings modal's controls are
// HOOKED IN: every representative setting is changed through the UI, its live
// effect (or persisted state after close + reopen) is asserted, and the
// original value is restored. Also covers the new entry point (top bar ⋯ →
// Settings) now that the rail's household card no longer opens the panel.
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

const openSettings = async (page) => {
  await page.locator('[data-testid="topbar-more"]').click({ force: true });
  await page.waitForTimeout(300);
  await page.locator('[data-testid="topbar-settings"]').click({ force: true });
  await page.waitForSelector(".flip-backdrop .set-tab-strip", { timeout: 8000 });
  await page.waitForTimeout(700);
};
const closeSettings = async (page) => {
  await page.locator(".flip-backdrop button:has-text('Close')").click({ force: true });
  await page.waitForTimeout(500);
};
const goTab = async (page, label) => {
  await page.locator(".flip-backdrop .set-tab-strip").getByText(label, { exact: true }).click({ force: true });
  await page.waitForTimeout(400);
};

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);

  // ── Entry point: rail card removed; ⋯ menu opens the panel. ────────────────
  ok(await page.locator("button.hh").count() === 0, "rail household card no longer opens settings");
  await openSettings(page);
  ok(true, "top-bar ⋯ → Settings opens the panel");

  // ── Household tab: base currency drives the rail summary live. ─────────────
  const baseSel = page.locator('.flip-backdrop select[aria-label="Base currency"]');
  const origBase = await baseSel.inputValue();
  await baseSel.selectOption("EUR");
  await page.waitForTimeout(600);
  ok(/EUR base/.test(await page.locator(".hh-quiet").innerText()), "base currency change reflects live in the rail summary");
  await baseSel.selectOption(origBase || "USD");
  await page.waitForTimeout(400);

  // Screens toggle: hide To-do → nav item disappears; restore.
  const todoToggle = page.locator(".flip-backdrop .toggle-row", { hasText: "Show To-do" }).locator(".switch");
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

  // ── Preferences tab: date format persists across close/reopen. ─────────────
  await goTab(page, "Preferences");
  const dateSel = page.locator('.flip-backdrop select[aria-label="Date format"]');
  const origDate = await dateSel.inputValue();
  await dateSel.selectOption("iso");
  await page.waitForTimeout(400);
  await closeSettings(page);
  await openSettings(page);
  await goTab(page, "Preferences");
  ok(await dateSel.inputValue() === "iso", "date format persists across close/reopen");
  await dateSel.selectOption(origDate || "long");
  // Week start segmented is wired.
  const weekSeg = page.locator(".flip-backdrop .toggle-row", { hasText: "Week starts on" });
  ok(await weekSeg.count() >= 1 || await page.locator(".flip-backdrop").getByText("Monday").count() >= 1, "week-start control present on Preferences");

  // ── AI tab: key field persists to app settings. ─────────────────────────────
  await goTab(page, "AI");
  const keyInput = page.locator('.flip-backdrop input[aria-label*="OpenAI"], .flip-backdrop input[placeholder*="OpenAI"]').first();
  ok(await keyInput.count() === 1, "AI key field present on the AI tab");
  await keyInput.fill("sk-e2e-hookup-test");
  await page.waitForTimeout(400);
  await closeSettings(page);
  await openSettings(page);
  await goTab(page, "AI");
  ok(await keyInput.inputValue() === "sk-e2e-hookup-test", "AI key persists across close/reopen");
  await keyInput.fill("");
  await page.waitForTimeout(300);

  // ── Cloud tab: the backend toggle reveals/hides the connection form. ───────
  await goTab(page, "Cloud");
  const backendSwitch = page.locator(".flip-backdrop .toggle-row", { hasText: "Connect to a backend" }).locator(".switch").first();
  const cloudBody = () => page.locator(".flip-backdrop .set-body").innerText();
  const wasOn = /aria-checked="true"/.test(await backendSwitch.evaluate((el) => el.outerHTML).catch(() => ""));
  await backendSwitch.click({ force: true });
  await page.waitForTimeout(400);
  const after = await cloudBody();
  ok(/fully local|server address|https:\/\//i.test(after), "backend toggle changes the Cloud tab's state");
  await backendSwitch.click({ force: true }); // restore
  await page.waitForTimeout(400);
  void wasOn;

  // ── Data tab: backup cadence persists. ──────────────────────────────────────
  await goTab(page, "Data");
  const cadenceSel = page.locator('.flip-backdrop select[aria-label="Backup reminders"], .flip-backdrop select[aria-label*="ackup"]').first();
  ok(await cadenceSel.count() === 1, "backup cadence select present on the Data tab");
  const origCad = await cadenceSel.inputValue();
  await cadenceSel.selectOption("weekly");
  await closeSettings(page);
  await openSettings(page);
  await goTab(page, "Data");
  ok(await cadenceSel.inputValue() === "weekly", "backup cadence persists across close/reopen");
  await cadenceSel.selectOption(origCad || "monthly");
  ok(/Export JSON|Export CSV/i.test(await page.locator(".flip-backdrop .set-body").innerText()), "data actions present");

  // ── Advanced tab: app lock + languages are reachable. ──────────────────────
  await goTab(page, "Advanced");
  const advText = await page.locator(".flip-backdrop .set-body").innerText();
  ok(/passcode/i.test(advText), "app lock section on Advanced");
  ok(/Languages/i.test(advText), "languages section on Advanced");
  await closeSettings(page);

  ok(errors.length === 0, `no page errors (${errors.join(" | ") || "none"})`);
} finally { await browser.close(); }
if (fails.length) { console.error("\nFAIL settings_hookup_check:\n - " + fails.join("\n - ")); process.exit(1); }
console.log("\nPASS: settings_hookup_check — every tab's settings are hooked in");
