// GX4 — Global accessibility audit probe.
// Tests: focus rings, unlabeled interactive elements, heading hierarchy,
// landmark structure, target sizes, outline suppression, aria-live regions.
// Screens: /, /transactions, /budgets, + the +Add modal.
// Themes: dark (default) and light at 1280px viewport.
//
// Output: JSON summary to stdout + screenshots in e2e/screenshots/
//
// Run: node e2e/gx_04_a11y.mjs
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

// ---------------------------------------------------------------------------
// accName — accessible-name computation (spec subset)
// ---------------------------------------------------------------------------
const ACC_NAME_FN = `
function accName(el) {
  const al = el.getAttribute("aria-label");
  if (al && al.trim()) return al.trim();
  const lb = el.getAttribute("aria-labelledby");
  if (lb) {
    const t = lb.split(/\\s+/).map(id => {
      const e = document.getElementById(id);
      return e ? (e.textContent || "").trim() : "";
    }).join(" ").trim();
    if (t) return t;
  }
  const title = el.getAttribute("title");
  if (title && title.trim()) return title.trim();
  if (el.id) {
    const esc = window.CSS && CSS.escape ? CSS.escape(el.id) : el.id;
    const lab = document.querySelector('label[for="' + esc + '"]');
    if (lab && (lab.textContent || "").trim()) return lab.textContent.trim();
  }
  // wrapping label
  const pl = el.closest("label");
  if (pl) {
    const clone = pl.cloneNode(true);
    clone.querySelectorAll("input,select,textarea,button").forEach(c => c.remove());
    const t2 = (clone.textContent || "").trim();
    if (t2) return t2;
  }
  // text content
  const tc = (el.textContent || "").trim();
  if (tc) return tc;
  // placeholder
  const ph = el.getAttribute("placeholder");
  if (ph && ph.trim()) return "[placeholder] " + ph.trim();
  // value on submit/button
  const val = el.getAttribute("value");
  if (val && val.trim()) return val.trim();
  // alt on img children
  const img = el.querySelector("img[alt]");
  if (img && img.alt.trim()) return img.alt.trim();
  return "";
}
`;

// ---------------------------------------------------------------------------
// findUnlabeled — return elements with no accessible name
// ---------------------------------------------------------------------------
const UNLABELED_FN = `
(function() {
  ${ACC_NAME_FN}
  const sel = 'button, a[href], input:not([type="hidden"]), select, textarea, [role="button"], [role="link"], [role="checkbox"], [role="switch"]';
  return [...document.querySelectorAll(sel)].filter(el => {
    const style = getComputedStyle(el);
    if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0) return false;
    return !accName(el);
  }).map(el => ({
    tag: el.tagName,
    type: el.type || "",
    class: el.className.toString().slice(0, 60),
    outerHTML: el.outerHTML.slice(0, 120)
  }));
})()
`;

// ---------------------------------------------------------------------------
// headings — extract all heading elements
// ---------------------------------------------------------------------------
const HEADINGS_FN = `
[...document.querySelectorAll("h1,h2,h3,h4,h5,h6,[role='heading']")].map(el => ({
  tag: el.tagName,
  level: el.getAttribute("aria-level") || el.tagName.replace("H",""),
  text: (el.textContent || "").trim().slice(0, 80),
  class: el.className.toString().slice(0, 40)
}))
`;

// ---------------------------------------------------------------------------
// landmarks — check presence of key landmark elements
// ---------------------------------------------------------------------------
const LANDMARKS_FN = `
({
  main: !!document.querySelector("main, [role='main']"),
  nav: !!document.querySelector("nav, [role='navigation']"),
  dialog: !!document.querySelector("[role='dialog']"),
  ariaLive: [...document.querySelectorAll("[aria-live]")].map(el => ({
    tag: el.tagName,
    live: el.getAttribute("aria-live"),
    class: el.className.toString().slice(0,40)
  })),
  skipLink: !!document.querySelector(".skip-link, a[href='#main'], a[href='#content']"),
  mainLabel: document.querySelector("main, [role='main']") ? (document.querySelector("main, [role='main']").getAttribute("aria-label") || document.querySelector("main, [role='main']").getAttribute("aria-labelledby") || null) : null
})
`;

// ---------------------------------------------------------------------------
// targetSizes — measure all buttons, links, icon buttons
// ---------------------------------------------------------------------------
const TARGET_SIZES_FN = `
(function() {
  const sel = 'button, a[href], [role="button"]';
  return [...document.querySelectorAll(sel)].filter(el => {
    const style = getComputedStyle(el);
    return style.display !== "none" && style.visibility !== "hidden";
  }).map(el => {
    const r = el.getBoundingClientRect();
    return {
      tag: el.tagName,
      class: el.className.toString().slice(0,60),
      w: Math.round(r.width),
      h: Math.round(r.height),
      text: (el.textContent || "").trim().slice(0,40),
      tooSmall: r.width < 44 || r.height < 44
    };
  }).filter(x => x.w > 0 || x.h > 0);
})()
`;

// ---------------------------------------------------------------------------
// focusRingCheck — tab and check computed outline on focused element
// ---------------------------------------------------------------------------
async function checkFocusRing(page, label) {
  // Press Tab to focus first interactive element
  await page.keyboard.press("Tab");
  const ring = await page.evaluate(() => {
    const el = document.activeElement;
    if (!el || el === document.body) return null;
    const cs = getComputedStyle(el);
    return {
      tag: el.tagName,
      class: el.className ? el.className.toString().slice(0,60) : "",
      outline: cs.outline,
      outlineWidth: cs.outlineWidth,
      outlineStyle: cs.outlineStyle,
      outlineColor: cs.outlineColor,
      outlineOffset: cs.outlineOffset,
      boxShadow: cs.boxShadow
    };
  });
  return ring;
}

// ---------------------------------------------------------------------------
// outlineSuppression — detect :focus outline: none / 0 in stylesheets
// ---------------------------------------------------------------------------
const OUTLINE_CHECK_FN = `
(function() {
  const suppressed = [];
  for (const sheet of document.styleSheets) {
    let rules;
    try { rules = sheet.cssRules; } catch(e) { continue; }
    if (!rules) continue;
    for (const rule of rules) {
      if (!rule.selectorText) continue;
      const sel = rule.selectorText.toLowerCase();
      if (sel.includes(":focus") || sel.includes(":focus-visible") || sel.includes(":focus-within")) {
        const ow = rule.style && rule.style.outlineWidth;
        const os = rule.style && rule.style.outlineStyle;
        const o = rule.style && rule.style.outline;
        if (o === "none" || o === "0" || ow === "0px" || os === "none") {
          suppressed.push({ selector: rule.selectorText, outline: o || "", outlineWidth: ow || "", outlineStyle: os || "" });
        }
      }
    }
  }
  return suppressed;
})()
`;

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------
const browser = await chromium.launch({ headless: true });
const screenshotFiles = [];
const summary = { screens: {}, themes: ["dark", "light"], issues: [] };

async function waitForApp(page) {
  // Wait until boot splash is hidden and #app has children (wasm mounted)
  await page.waitForFunction(() => {
    const app = document.getElementById("app");
    if (!app || app.children.length === 0) return false;
    const boot = document.getElementById("boot");
    if (!boot) return true;
    const cs = getComputedStyle(boot);
    return cs.display === "none" || Number(cs.opacity) === 0 || boot.classList.contains("hidden");
  }, { timeout: 120000 });
  // Extra settle time for wasm + transitions
  await page.waitForTimeout(1500);
}

async function auditScreen(page, screenKey, theme, route) {
  const label = `${screenKey}_${theme}_1280`;
  const findings = { route, theme, screenKey };

  // Unlabeled elements
  findings.unlabeled = await page.evaluate(new Function(`return (${UNLABELED_FN})`));

  // Headings
  findings.headings = await page.evaluate(HEADINGS_FN);

  // Landmarks
  findings.landmarks = await page.evaluate(`(${LANDMARKS_FN})`);

  // Target sizes — only flag elements under 44px
  const sizes = await page.evaluate(`(${TARGET_SIZES_FN})`);
  findings.smallTargets = sizes.filter(x => x.tooSmall);
  findings.totalButtons = sizes.length;

  // Outline suppression in CSS
  findings.outlineSuppressed = await page.evaluate(`(${OUTLINE_CHECK_FN})`);

  // Focus ring: tab through 5 stops, screenshot each
  findings.focusRings = [];
  for (let i = 1; i <= 5; i++) {
    await page.keyboard.press("Tab");
    const ring = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el || el === document.body) return null;
      const cs = getComputedStyle(el);
      return {
        tag: el.tagName,
        class: el.className ? el.className.toString().slice(0,60) : "",
        outline: cs.outline,
        outlineWidth: cs.outlineWidth,
        outlineStyle: cs.outlineStyle,
        outlineColor: cs.outlineColor,
        outlineOffset: cs.outlineOffset
      };
    });
    const fname = `gx04_focus_${label}_tab${i}.png`;
    const fpath = path.join(SHOTS, fname);
    await page.screenshot({ path: fpath, fullPage: false });
    screenshotFiles.push(fname);
    if (ring) {
      const hasRing = ring.outlineWidth !== "0px" && ring.outlineStyle !== "none" && ring.outline !== "none" && ring.outline !== "";
      findings.focusRings.push({ tab: i, ...ring, hasRing });
    }
  }

  // Full-page screenshot
  const fname2 = `gx04_${label}.png`;
  await page.screenshot({ path: path.join(SHOTS, fname2), fullPage: true });
  screenshotFiles.push(fname2);

  return findings;
}

const THEMES = [
  { name: "dark", setup: null },
  {
    name: "light",
    setup: async (page) => {
      // Set theme in localStorage BEFORE the WASM reads it (app already loaded at this point)
      await page.evaluate(() => {
        localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }));
        localStorage.setItem("cashflux:theme", JSON.stringify("light"));
        // Trigger theme engine if it exposes a setter
        if (window.__cf_setTheme) window.__cf_setTheme("light");
        // Also dispatch a storage event to notify listeners
        window.dispatchEvent(new StorageEvent("storage", { key: "cashflux:prefs" }));
      });
      // Reload so the app initializes with light theme from localStorage
      await page.reload({ waitUntil: "domcontentloaded" });
      await waitForApp(page);
      // Wait until data-theme=light is set (may take a moment after wasm boot)
      await page.waitForFunction(
        () => document.documentElement.getAttribute("data-theme") === "light",
        { timeout: 15000 }
      ).catch(() => {});
    }
  }
];

const SCREENS = [
  { key: "dashboard", route: "/" },
  { key: "transactions", route: "/transactions" },
  { key: "budgets", route: "/budgets" },
];

for (const theme of THEMES) {
  for (const screen of SCREENS) {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    page.setDefaultTimeout(120000);

    // Navigate to root first so WASM loads from cache, then pushState to target
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await waitForApp(page);

    // Apply theme if needed (sets localStorage before WASM has loaded)
    if (theme.setup) {
      await theme.setup(page);
    }

    // Navigate to target route via pushState (SPA-style, no full reload)
    if (screen.route !== "/") {
      await page.evaluate((r) => window.history.pushState({}, "", r), screen.route);
      await page.evaluate(() => window.dispatchEvent(new PopStateEvent("popstate")));
      await page.waitForTimeout(1200);
    }

    // Small wait for any dynamic rendering
    await page.waitForTimeout(800);

    const findings = await auditScreen(page, screen.key, theme.name, screen.route);
    const key = `${screen.key}_${theme.name}`;
    summary.screens[key] = findings;
    await ctx.close();
  }

  // +Add modal audit (dark only to avoid duplication)
  if (theme.name === "dark") {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    page.setDefaultTimeout(120000);
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await waitForApp(page);
    await page.evaluate(() => window.history.pushState({}, "", "/transactions"));
    await page.evaluate(() => window.dispatchEvent(new PopStateEvent("popstate")));
    await page.waitForTimeout(1200);
    await page.waitForTimeout(600);

    // Try to open +Add modal via the add button
    try {
      const addBtn = await page.$(".add-btn");
      if (addBtn) {
        await addBtn.click();
        await page.waitForTimeout(600);
        // Click first add-item (Transaction)
        const addItem = await page.$(".add-item");
        if (addItem) {
          await addItem.click();
          await page.waitForTimeout(800);
        }
      }
    } catch (e) { /* modal may not open */ }

    const findings = await auditScreen(page, "add_modal", "dark", "/transactions+modal");
    summary.screens["add_modal_dark"] = findings;

    // Screenshot of modal state
    const fname = "gx04_add_modal_dark_1280.png";
    await page.screenshot({ path: path.join(SHOTS, fname), fullPage: false });
    screenshotFiles.push(fname);

    await ctx.close();
  }
}

await browser.close();

// ---------------------------------------------------------------------------
// Compile top-level issues list
// ---------------------------------------------------------------------------
let totalUnlabeled = 0;
let totalSmallTargets = 0;
let outlineSuppressedFound = [];
let missingLandmarks = [];
let focusRingFailures = [];

for (const [key, f] of Object.entries(summary.screens)) {
  totalUnlabeled += f.unlabeled ? f.unlabeled.length : 0;
  totalSmallTargets += f.smallTargets ? f.smallTargets.length : 0;
  if (f.outlineSuppressed && f.outlineSuppressed.length > 0) {
    outlineSuppressedFound.push(...f.outlineSuppressed.map(x => ({ screen: key, ...x })));
  }
  if (f.landmarks) {
    if (!f.landmarks.main) missingLandmarks.push({ screen: key, missing: "main" });
    if (!f.landmarks.nav) missingLandmarks.push({ screen: key, missing: "nav" });
  }
  if (f.focusRings) {
    const noRing = f.focusRings.filter(r => !r.hasRing);
    if (noRing.length > 0) focusRingFailures.push({ screen: key, noRing });
  }
}

summary.totals = {
  unlabeledInteractiveElements: totalUnlabeled,
  smallTargetsUnder44px: totalSmallTargets,
  outlineSuppressedRules: outlineSuppressedFound.length,
  missingLandmarks: missingLandmarks.length,
  focusRingFailures: focusRingFailures.length,
  screenshotsProduced: screenshotFiles.length,
  screenshotFiles
};

summary.outlineSuppressedRules = outlineSuppressedFound;
summary.missingLandmarks = missingLandmarks;
summary.focusRingFailures = focusRingFailures;

console.log(JSON.stringify(summary, null, 2));
