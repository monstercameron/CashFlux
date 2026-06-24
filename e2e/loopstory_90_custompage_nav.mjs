// LOOPSTORY 90 — Custom-page → custom-page navigation regression.
//
// The bug: clicking one custom page in the rail and then another *directly*
// updated the URL and the top bar title but left the BODY showing the previous
// custom page. Every custom page renders through the same screens.CustomPage
// component, and its "/p/:slug" View closure is created at one source line, so all
// custom pages share a function code-pointer. The reconciler saw the same element
// type with equal props and skipped re-rendering the page subtree. Going through a
// built-in page first worked because the component type changed, forcing a remount.
//
// The fix: the Shell renders the active screen as WithKey(CreateElement(view),
// activePath) — a per-route key gives each navigation a distinct element identity,
// so the reconciler unmounts the old page and mounts the new one every time. This
// test creates two distinct custom pages and asserts the body swaps on a direct
// custom→custom hop (both directions), and that built-in pages stay distinct.
//
// NOTE: a separate, pre-existing issue logs one "call to released function" console
// error per navigation across the WHOLE app (built-in routes too). It is unrelated
// to this fix (present before it on every route change), so it is reported here for
// visibility but does NOT gate this regression.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const consoleErrors = [];

async function dismissOverlay(page) {
  await page.evaluate(() => {
    const o = document.getElementById("gwc-error-overlay") || document.querySelector(".gwc-error-overlay");
    if (o) o.remove();
  });
}

async function createCustomPage(page, name) {
  await dismissOverlay(page);
  const newPageEl = page.locator("nav a, nav button").filter({ hasText: /new page/i }).first();
  if (await newPageEl.count() === 0) throw new Error("'New page' rail action not found");
  await newPageEl.click();
  await page.waitForSelector(".cf-dialog-input", { timeout: 10000 });
  await page.evaluate((n) => {
    const inp = document.querySelector(".cf-dialog-input, #cf-dialog-input");
    if (inp) { inp.value = n; inp.dispatchEvent(new Event("input", { bubbles: true })); inp.dispatchEvent(new Event("change", { bubbles: true })); }
  }, name);
  await page.waitForTimeout(200);
  await page.evaluate(() => { const b = document.querySelector("#cf-dialog-confirm"); if (b) b.click(); });
  await page.waitForTimeout(1500);
  const m = page.url().match(/\/p\/([^/?#]+)/);
  return m ? m[1] : null;
}

// A widget whose title is rendered in the tile header — a unique, visible marker
// that proves WHICH custom page's body is mounted.
async function addKpiWidget(page, title) {
  await page.locator("button").filter({ hasText: /^add widget$/i }).first().click();
  await page.waitForSelector("select.field", { timeout: 5000 });
  await page.locator("select.field").first().selectOption("kpi");
  await page.waitForTimeout(150);
  await page.locator("input.field[placeholder]").first().fill(title);
  await page.waitForTimeout(150);
  const submit = page.locator(".card button.btn-primary").filter({ hasText: /^add$/i }).first();
  if (await submit.count() > 0) await submit.click();
  else await page.locator("button.btn-primary").filter({ hasText: /add/i }).first().click();
  await page.waitForTimeout(1000);
}

const bodyText = (page) => page.evaluate(() => {
  const el = document.getElementById("cf-page-view");
  return el ? el.textContent.replace(/\s+/g, " ").trim() : "(no #cf-page-view)";
});

async function navCustom(page, name) {
  // Custom-page rail links carry a title attribute but no href.
  const link = page.locator(`nav a[title="${name}"]`).first();
  if (await link.count() === 0) throw new Error("custom rail link not found: " + name);
  await link.click();
  await page.waitForTimeout(900);
}

async function navBuiltin(page, href) {
  await page.locator(`nav a[href="${href}"]`).first().click();
  await page.waitForTimeout(700);
}

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.on("pageerror", (e) => consoleErrors.push("pageerror: " + e.message));
page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });

await page.setViewportSize({ width: 1280, height: 900 });
await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(1500);

console.log("Creating two custom pages with distinct widgets…");
await createCustomPage(page, "Alpha Page");
await addKpiWidget(page, "ALPHAWIDGET");
await createCustomPage(page, "Beta Page");
await addKpiWidget(page, "BETAWIDGET");

console.log("=== custom → custom (the regression) ===");
await navCustom(page, "Alpha Page");
const a1 = await bodyText(page);
const onA = a1.includes("ALPHAWIDGET") && !a1.includes("BETAWIDGET");

await navCustom(page, "Beta Page"); // direct custom→custom — the broken hop
const b1 = await bodyText(page);
const swapToB = b1.includes("BETAWIDGET") && !b1.includes("ALPHAWIDGET");

await navCustom(page, "Alpha Page"); // and back
const a2 = await bodyText(page);
const swapBackToA = a2.includes("ALPHAWIDGET") && !a2.includes("BETAWIDGET");

console.log("=== built-in pages stay distinct ===");
await navBuiltin(page, "/accounts");
const accts = await bodyText(page);
await navBuiltin(page, "/transactions");
const txns = await bodyText(page);
await navBuiltin(page, "/");
const dash = await bodyText(page);
const builtinDistinct = accts !== txns && txns !== dash && accts.length > 0 && txns.length > 0;

const releasedFn = consoleErrors.filter((e) => /call to released function/.test(e));
const otherErrors = consoleErrors.filter((e) => !/call to released function/.test(e));

console.log("\n=== RESULTS ===");
console.log("custom A shows A:        ", onA);
console.log("custom A→B swaps to B:   ", swapToB);
console.log("custom B→A swaps back:   ", swapBackToA);
console.log("built-in pages distinct: ", builtinDistinct);
console.log(`pre-existing nav errors:  ${releasedFn.length} × "call to released function" (NOT gating — app-wide, predates this fix)`);
console.log("other console errors:    ", otherErrors.length === 0 ? "none" : otherErrors);

await browser.close();
const pass = onA && swapToB && swapBackToA && builtinDistinct && otherErrors.length === 0;
console.log("\nOVERALL:", pass ? "PASS" : "FAIL");
process.exit(pass ? 0 : 1);
