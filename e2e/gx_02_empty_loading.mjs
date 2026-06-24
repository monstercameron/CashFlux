// GX2 — Empty & loading states visual review.
// Story: "The First Run / The Empty Month" — what does a brand-new user (or any
// user at an empty period like 2099-01) actually see on each screen?
//
// Captures screenshots at 1280x800 and 768x1024 in dark and light themes for:
//   • Boot splash (pre-app)
//   • Each major screen at period 2099-01 (zero data state)
//   • Transactions filtered with "zzzzz" (filtered-empty state)
//   • Naturally-empty screens (Artifacts, Workflows, Documents)
//
// Also measures text contrast on empty state elements.
// Saves to e2e/screenshots/ with prefix gx02_.
// Exit code 0 — evidence-harvest script, not a pass/fail gate.

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS_DIR = path.join(__dirname, "screenshots");
fs.mkdirSync(SHOTS_DIR, { recursive: true });

const WIDTHS = [1280, 768];
const HEIGHTS = { 1280: 800, 768: 1024 };

// Boot a fresh page with the requested theme pre-set in localStorage.
// Pass wipe=true to clear sample data (so all screens show genuine empty states).
async function bootWithTheme(browser, width, theme, wipe = false) {
  const ctx = await browser.newContext({
    viewport: { width, height: HEIGHTS[width] },
  });
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });

  // Set both the standalone atom and the prefs blob so the app picks up the theme.
  // If wipe=true, clear the dataset and set the seeded flag so the app treats
  // the empty localStorage as an intentional clean slate (not a first run that
  // would re-seed sample data).
  await page.evaluate(({ t, wipe }) => {
    localStorage.setItem("cashflux:theme", JSON.stringify(t));
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      if (raw) {
        const p = JSON.parse(raw);
        p.theme = t;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      }
    } catch (_) {}
    if (wipe) {
      localStorage.removeItem("cashflux:dataset");
      localStorage.setItem("cashflux:seeded", "1"); // tell app this is intentional
      localStorage.removeItem("cashflux:sampleActive");
    }
  }, { t: theme, wipe });

  // Hard reload so WASM boots with the theme applied.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('aside.rail, nav[aria-label="Main navigation"]', {
    timeout: 60000,
  });
  await page.waitForTimeout(1200); // settle WASM + transitions

  return { page, ctx, errors };
}

// Save a screenshot and log the filename.
async function shot(page, name) {
  const p = path.join(SHOTS_DIR, name);
  await page.screenshot({ path: p, fullPage: false });
  console.log(`  shot: ${name}`);
  return name;
}

// Navigate to a route and wait for the app to settle.
async function nav(page, route) {
  await page.evaluate((r) => window.history.pushState({}, "", r), route);
  // Trigger a popstate so the router picks it up.
  await page.evaluate(() => window.dispatchEvent(new PopStateEvent("popstate")));
  await page.waitForTimeout(800);
}

// Measure contrast-relevant CSS on the first .empty element found.
async function measureEmpty(page) {
  return page.evaluate(() => {
    const el = document.querySelector(".empty");
    if (!el) return { _missing: true };
    const cs = getComputedStyle(el);
    return {
      color: cs.color,
      "background-color": cs.backgroundColor,
      "font-size": cs.fontSize,
      "font-weight": cs.fontWeight,
      opacity: cs.opacity,
    };
  });
}

// Measure the .empty-cta block if present.
async function measureEmptyCTA(page) {
  return page.evaluate(() => {
    const el = document.querySelector(".empty-cta");
    if (!el) return { _missing: true };
    const cs = getComputedStyle(el);
    const btn = el.querySelector("button");
    const btnCS = btn ? getComputedStyle(btn) : null;
    return {
      display: cs.display,
      "align-items": cs.alignItems,
      gap: cs.gap,
      hasIcon: !!el.querySelector("svg"),
      hasMessage: !!el.querySelector("p.empty"),
      hasButton: !!btn,
      btnText: btn ? btn.textContent.trim() : null,
      btnBg: btnCS ? btnCS.backgroundColor : null,
      btnColor: btnCS ? btnCS.color : null,
    };
  });
}

const browser = await chromium.launch({ headless: true });
const report = { screenshots: [], measurements: {} };

// ── Step 1: Boot splash (grab before WASM loads by using a fresh context) ──────
console.log("\n=== Boot splash ===");
{
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const splashPage = await ctx.newPage();
  // Intercept wasm so the splash lingers.
  await splashPage.route("**/*.wasm", (route) => new Promise(() => {})); // never fulfill
  await splashPage.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await splashPage.waitForSelector("#boot", { timeout: 10000 });
  await shot(splashPage, "gx02_boot_splash_1280_dark.png");
  report.screenshots.push("gx02_boot_splash_1280_dark.png");
  // Measure splash styles.
  const splashM = await splashPage.evaluate(() => {
    const card = document.querySelector(".boot-card");
    const sub = document.querySelector(".boot-sub");
    if (!card) return { _missing: true };
    const cs = getComputedStyle(card);
    return {
      background: getComputedStyle(document.getElementById("boot")).backgroundColor,
      cardBg: cs.backgroundColor,
      subText: sub ? sub.textContent.trim() : null,
      hasRing: !!document.querySelector(".boot-ring"),
      hasWord: !!document.querySelector(".boot-word"),
    };
  });
  console.log("  splash:", JSON.stringify(splashM));
  report.measurements["splash"] = splashM;
  await ctx.close();
}

// ── Step 2: Per-screen empty state captures (dark theme, both widths) ──────────
const SCREENS = [
  { route: "/", label: "dashboard" },
  { route: "/transactions", label: "transactions" },
  { route: "/accounts", label: "accounts" },
  { route: "/budgets", label: "budgets" },
  { route: "/goals", label: "goals" },
  { route: "/todo", label: "todo" },
  { route: "/planning", label: "planning" },
  { route: "/allocate", label: "allocate" },
  { route: "/reports", label: "reports" },
  { route: "/insights", label: "insights" },
  { route: "/documents", label: "documents" },
  { route: "/artifacts", label: "artifacts" },
  { route: "/workflows", label: "workflows" },
  { route: "/bills", label: "bills" },
  { route: "/subscriptions", label: "subscriptions" },
  { route: "/split", label: "split" },
  { route: "/members", label: "members" },
  { route: "/categories", label: "categories" },
  { route: "/rules", label: "rules" },
  { route: "/customize", label: "customize" },
];

for (const width of WIDTHS) {
  console.log(`\n=== dark ${width} — empty-state tour (wiped) ===`);
  const { page, ctx, errors } = await bootWithTheme(browser, width, "dark", true);

  for (const screen of SCREENS) {
    console.log(`  → ${screen.route}`);
    await nav(page, screen.route);
    const name = `gx02_${screen.label}_empty_dark_${width}.png`;
    await shot(page, name);
    report.screenshots.push(name);

    const emptyM = await measureEmpty(page);
    const ctaM = await measureEmptyCTA(page);
    report.measurements[`${screen.label}_empty_dark_${width}`] = {
      emptyText: emptyM,
      emptyCTA: ctaM,
    };
  }

  // Filtered-empty: transactions with search "zzzzz"
  console.log("  → /transactions?q=zzzzz (filtered-empty)");
  await nav(page, "/transactions");
  // Try to find the search input and type zzzzz.
  const searchSel = 'input[type="search"], input[placeholder*="earch"], input[aria-label*="earch"], input[aria-label*="ilter"]';
  const hasSearch = await page.$(searchSel);
  if (hasSearch) {
    await page.fill(searchSel, "zzzzz");
    await page.waitForTimeout(600);
  } else {
    // Try clicking a search/filter button first.
    const filterBtn = await page.$('button[aria-label*="ilter"], button[title*="earch"], button[aria-label*="earch"]');
    if (filterBtn) {
      await filterBtn.click();
      await page.waitForTimeout(400);
      const inp = await page.$(searchSel);
      if (inp) { await page.fill(searchSel, "zzzzz"); await page.waitForTimeout(600); }
    }
  }
  const filteredName = `gx02_transactions_filtered_dark_${width}.png`;
  await shot(page, filteredName);
  report.screenshots.push(filteredName);

  if (errors.length) console.warn(`  page errors (dark ${width}):`, errors.join(" | "));
  await ctx.close();
}

// ── Step 3: Light theme — key empty screens ────────────────────────────────────
const LIGHT_SCREENS = [
  { route: "/", label: "dashboard" },
  { route: "/transactions", label: "transactions" },
  { route: "/accounts", label: "accounts" },
  { route: "/budgets", label: "budgets" },
  { route: "/goals", label: "goals" },
  { route: "/artifacts", label: "artifacts" },
  { route: "/workflows", label: "workflows" },
];

for (const width of WIDTHS) {
  console.log(`\n=== light ${width} — empty-state tour (wiped) ===`);
  const { page, ctx, errors } = await bootWithTheme(browser, width, "light", true);

  for (const screen of LIGHT_SCREENS) {
    console.log(`  → ${screen.route}`);
    await nav(page, screen.route);
    const name = `gx02_${screen.label}_empty_light_${width}.png`;
    await shot(page, name);
    report.screenshots.push(name);

    const emptyM = await measureEmpty(page);
    const ctaM = await measureEmptyCTA(page);
    report.measurements[`${screen.label}_empty_light_${width}`] = {
      emptyText: emptyM,
      emptyCTA: ctaM,
    };
  }

  if (errors.length) console.warn(`  page errors (light ${width}):`, errors.join(" | "));
  await ctx.close();
}

// ── Save report JSON ───────────────────────────────────────────────────────────
const reportPath = path.join(SHOTS_DIR, "gx02_measurements.json");
fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
console.log(`\nMeasurements written to ${reportPath}`);
console.log(`Total screenshots: ${report.screenshots.length}`);
console.log("Done. Exit 0.");

await browser.close();
