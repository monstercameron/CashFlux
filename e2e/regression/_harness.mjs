// _harness.mjs — shared helpers for the v1.0 regression + visual-inspection
// suite. Fresh browser context (blocks service workers so a stale cached wasm
// can't mask a change), first-run seed wait, SPA navigation, theme switching
// via the real Appearance control, screenshot + console-error capture.
//
// Usage:
//   import { boot, nav, setTheme, shot, jsClick, errsOf } from "./_harness.mjs";
//   const { browser, context, page, errors } = await boot();
//   await nav(page, "/transactions");
//   await setTheme(page, "light"); ... await setTheme(page, "dark");
//   await shot(page, "transactions_dark");
//   await context.close(); await browser.close();
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const require = createRequire(path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "..", ".tools", "package.json"));
const { chromium } = require("playwright");

export const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
export const SHOTS = process.env.SHOTS_DIR ||
  "C:/Users/mreca/AppData/Local/Temp/claude/C--Users-mreca-Desktop/5aacab8d-c372-4a7d-97dc-bfed206563c6/scratchpad/v1shots";

// boot launches a fresh headless context (GPU off avoids a headless paint quirk
// that produced all-black screenshots), waits for #app + first-run seed, and
// wires a page-error/console-error collector.
export async function boot(opts = {}) {
  const browser = await chromium.launch({ headless: true, args: ["--disable-gpu"] });
  const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
  const page = await context.newPage();
  await page.setViewportSize(opts.viewport || { width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push("pageerror: " + String(e).slice(0, 200)));
  page.on("console", (m) => { if (m.type() === "error") errors.push("console: " + m.text().slice(0, 200)); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 90000 });
  await page.waitForTimeout(opts.settle || 5500); // first-run seed + first paint
  return { browser, context, page, errors };
}

// nav SPA-navigates (deep URLs 404 on the dev server) and settles.
export async function nav(page, route, wait = 2000) {
  await page.evaluate((r) => { history.pushState({}, "", r); dispatchEvent(new PopStateEvent("popstate")); }, route);
  await page.waitForTimeout(wait);
}

// setTheme flips to "light" or "dark" via the /settings Appearance mode control
// (the honest path — the theme contract was just reworked). Leaves you on
// whatever route you navigate to next.
export async function setTheme(page, mode) {
  const want = mode === "light" ? "Light" : "Dark";
  const cur = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  if (cur === mode) return;
  await nav(page, "/settings", 1500);
  await page.evaluate(() => {
    const strip = document.querySelector(".settings-page .set-tab-strip");
    const t = [...(strip ? strip.querySelectorAll("button") : [])].find((b) => b.textContent.trim() === "Appearance");
    if (t) t.click();
  });
  await page.waitForTimeout(600);
  await page.evaluate((w) => {
    const seg = document.querySelector("#sec-appearance-mode");
    const b = [...(seg ? seg.querySelectorAll("button") : [])].find((x) => x.textContent.trim() === w);
    if (b) b.click();
  }, want);
  await page.waitForTimeout(700);
}

// jsClick clicks by a selector or a text match inside a scope, via raw JS
// (headless trusted clicks are flaky on these surfaces — a documented artifact).
export async function jsClick(page, arg) {
  return page.evaluate((a) => {
    let el;
    if (a.testid) el = document.querySelector(`[data-testid="${a.testid}"]`);
    else if (a.sel) el = document.querySelector(a.sel);
    else if (a.text) el = [...document.querySelectorAll(a.scope || "button")].find((b) => b.textContent.trim() === a.text);
    if (!el) return "NOT_FOUND";
    el.click();
    return "ok";
  }, arg);
}

// shot writes a full-page screenshot under SHOTS.
export async function shot(page, name) {
  await page.screenshot({ path: `${SHOTS}/${name}.png` });
  return `${SHOTS}/${name}.png`;
}

// text returns the main pane's innerText (case-insensitive matching downstream —
// innerText honors CSS text-transform).
export async function mainText(page) {
  return page.locator("#main").innerText();
}

export function errsOf(errors) {
  return errors.length ? errors.join(" || ") : "none";
}
