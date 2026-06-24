// GX14 verification — confirms that `[data-theme="light"]` !important tokens
// correctly override the inline-dark values ApplyTheme writes to <html>.style,
// and that dark mode is NOT regressed.
//
// Run: node e2e/gx14_verify.mjs
// Exit 0 = all checks passed; Exit 1 = at least one failure.

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { ready } from "./_ready.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS_DIR = path.join(__dirname, "screenshots");

const browser = await chromium.launch({ headless: true });
let failures = 0;

function fail(msg) {
  console.error("FAIL: " + msg);
  failures++;
}
function pass(msg) {
  console.log("PASS: " + msg);
}

// Parse a CSS color string to [r,g,b] or null.
// Handles: rgb(...), rgba(...), #rrggbb, #rgb
function parseRGB(color) {
  if (!color) return null;
  const m4 = color.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
  if (m4) return [Number(m4[1]), Number(m4[2]), Number(m4[3])];
  const h6 = color.match(/^#([0-9a-f]{6})$/i);
  if (h6) {
    const n = parseInt(h6[1], 16);
    return [(n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff];
  }
  const h3 = color.match(/^#([0-9a-f]{3})$/i);
  if (h3) {
    const [r, g, b] = h3[1].split("").map((c) => parseInt(c + c, 16));
    return [r, g, b];
  }
  return null;
}

// Is this color "light" (all channels >= 180)?
function isLight(color) {
  const rgb = parseRGB(color);
  if (!rgb) return null;
  return rgb[0] >= 180 && rgb[1] >= 180 && rgb[2] >= 180;
}

// Is this color "dark" (all channels < 80)?
function isDark(color) {
  const rgb = parseRGB(color);
  if (!rgb) return null;
  return rgb[0] < 80 && rgb[1] < 80 && rgb[2] < 80;
}

// ─── helpers ────────────────────────────────────────────────────────────────

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: t }));
  }, theme);
  await page.reload();
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute("data-theme") === t,
    theme,
    { timeout: 15_000 }
  );
  // Remove gwc error overlay if present
  await page.evaluate(() => {
    const ov = document.getElementById("gwc-error-overlay");
    if (ov) ov.remove();
  });
}

async function getCSSVar(page, varName) {
  return page.evaluate(
    (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim(),
    varName
  );
}

async function getElBg(page, selector) {
  try {
    const el = await page.$(selector);
    if (!el) return null;
    return page.evaluate((el) => getComputedStyle(el).backgroundColor, el);
  } catch {
    return null;
  }
}

// ─── SECTION 1: Light mode CSS vars on /dashboard ───────────────────────────

console.log("\n=== SECTION 1: Light mode CSS variables ===");

{
  const page = await browser.newPage();
  await page.goto(BASE + "/dashboard");
  await ready(page);
  await setTheme(page, "light");
  await ready(page);

  const bgElev = await getCSSVar(page, "--bg-elev");
  const bgCard = await getCSSVar(page, "--bg-card");
  const text = await getCSSVar(page, "--text");

  console.log(`  --bg-elev = "${bgElev}"`);
  console.log(`  --bg-card = "${bgCard}"`);
  console.log(`  --text    = "${text}"`);

  // --bg-elev should be light (≈ #efede8 = rgb(239,237,232))
  const elevLight = isLight(bgElev);
  if (elevLight === true) pass("--bg-elev is light");
  else if (elevLight === false) fail(`--bg-elev is DARK in light mode: "${bgElev}"`);
  else console.log(`  WARN: --bg-elev could not be parsed as rgb: "${bgElev}"`);

  // --bg-card should be ~white (rgb(255,255,255))
  const cardLight = isLight(bgCard);
  if (cardLight === true) pass("--bg-card is light");
  else if (cardLight === false) fail(`--bg-card is DARK in light mode: "${bgCard}"`);
  else console.log(`  WARN: --bg-card could not be parsed: "${bgCard}"`);

  // --text should be dark (≈ #1c1c1e)
  const textDark = isDark(text);
  if (textDark === true) pass("--text is dark (correct for light mode)");
  else if (textDark === false) fail(`--text is LIGHT in light mode (near-white text): "${text}"`);
  else console.log(`  WARN: --text could not be parsed: "${text}"`);

  await page.screenshot({ path: path.join(SHOTS_DIR, "gx14_verify_dashboard_light.png"), fullPage: true });
  console.log("  Screenshot: gx14_verify_dashboard_light.png");

  await page.close();
}

// ─── SECTION 2: Real element backgrounds across pages ───────────────────────

console.log("\n=== SECTION 2: Real element backgrounds (light mode) ===");

// /dashboard
{
  const page = await browser.newPage();
  await page.goto(BASE + "/dashboard");
  await ready(page);
  await setTheme(page, "light");
  await ready(page);

  const tileBg = await getElBg(page, ".w");
  const topbarBg = await getElBg(page, ".topbar");

  if (tileBg !== null) {
    const ok = isLight(tileBg);
    if (ok === true) pass(`/dashboard .w tile bg = "${tileBg}"`);
    else if (ok === false) fail(`/dashboard .w tile is DARK in light mode: "${tileBg}"`);
    else console.log(`  WARN: .w bg could not be parsed: "${tileBg}"`);
  } else {
    console.log("  WARN: /dashboard .w not found");
  }

  if (topbarBg !== null) {
    const ok = isLight(topbarBg);
    if (ok === true) pass(`/dashboard .topbar bg = "${topbarBg}"`);
    else if (ok === false) fail(`/dashboard .topbar is DARK in light mode: "${topbarBg}"`);
    else console.log(`  WARN: .topbar bg could not be parsed: "${topbarBg}"`);
  } else {
    console.log("  WARN: /dashboard .topbar not found");
  }

  await page.close();
}

// /budgets
{
  const page = await browser.newPage();
  await page.goto(BASE + "/budgets");
  await ready(page);
  await setTheme(page, "light");
  await ready(page);

  const barBg = await getElBg(page, ".bar");
  const statBg = await getElBg(page, ".stat");

  await page.screenshot({ path: path.join(SHOTS_DIR, "gx14_verify_budgets_light.png"), fullPage: true });
  console.log("  Screenshot: gx14_verify_budgets_light.png");

  if (barBg !== null) {
    const ok = isLight(barBg);
    if (ok === true) pass(`/budgets .bar track bg = "${barBg}"`);
    else if (ok === false) fail(`/budgets .bar is DARK in light mode: "${barBg}"`);
    else console.log(`  WARN: .bar bg could not be parsed: "${barBg}"`);
  } else {
    console.log("  WARN: /budgets .bar not found");
  }

  if (statBg !== null) {
    const ok = isLight(statBg);
    if (ok === true) pass(`/budgets .stat tile bg = "${statBg}"`);
    else if (ok === false) fail(`/budgets .stat is DARK in light mode: "${statBg}"`);
    else console.log(`  WARN: .stat bg could not be parsed: "${statBg}"`);
  } else {
    console.log("  WARN: /budgets .stat not found");
  }

  await page.close();
}

// /transactions
{
  const page = await browser.newPage();
  await page.goto(BASE + "/transactions");
  await ready(page);
  await setTheme(page, "light");
  await ready(page);

  const thBg = await getElBg(page, ".txn-table thead th");
  const filterBg = await getElBg(page, ".txn-table select, select");

  await page.screenshot({ path: path.join(SHOTS_DIR, "gx14_verify_transactions_light.png"), fullPage: true });
  console.log("  Screenshot: gx14_verify_transactions_light.png");

  if (thBg !== null) {
    const ok = isLight(thBg);
    if (ok === true) pass(`/transactions .txn-table thead th bg = "${thBg}"`);
    else if (ok === false) fail(`/transactions thead th is DARK in light mode: "${thBg}"`);
    else console.log(`  WARN: thead th bg could not be parsed: "${thBg}"`);
  } else {
    console.log("  WARN: /transactions .txn-table thead th not found");
  }

  if (filterBg !== null) {
    const ok = isLight(filterBg);
    if (ok === true) pass(`/transactions filter select bg = "${filterBg}"`);
    else if (ok === false) fail(`/transactions filter select is DARK in light mode: "${filterBg}"`);
    else console.log(`  WARN: filter select bg could not be parsed: "${filterBg}"`);
  } else {
    console.log("  WARN: /transactions select not found");
  }

  await page.close();
}

// /accounts or /goals — .card
{
  const page = await browser.newPage();
  await page.goto(BASE + "/accounts");
  await ready(page);
  await setTheme(page, "light");
  await ready(page);

  let cardBg = await getElBg(page, ".card");
  if (cardBg === null) {
    // try /goals
    await page.goto(BASE + "/goals");
    await ready(page);
    await setTheme(page, "light");
    await ready(page);
    cardBg = await getElBg(page, ".card");
  }

  if (cardBg !== null) {
    const ok = isLight(cardBg);
    if (ok === true) pass(`/accounts|/goals .card bg = "${cardBg}"`);
    else if (ok === false) fail(`.card is DARK in light mode: "${cardBg}"`);
    else console.log(`  WARN: .card bg could not be parsed: "${cardBg}"`);
  } else {
    console.log("  WARN: .card not found on /accounts or /goals");
  }

  await page.close();
}

// ─── SECTION 3: Dark mode regression ────────────────────────────────────────

console.log("\n=== SECTION 3: Dark mode regression ===");

{
  const page = await browser.newPage();
  await page.goto(BASE + "/dashboard");
  await ready(page);
  await setTheme(page, "dark");
  await ready(page);

  const bgCard = await getCSSVar(page, "--bg-card");
  const cardBg = await getElBg(page, ".card, .w");
  const thBg = await getElBg(page, ".txn-table thead th");

  console.log(`  dark --bg-card    = "${bgCard}"`);
  console.log(`  dark .card/.w bg  = "${cardBg}"`);
  console.log(`  dark thead th bg  = "${thBg}"`);

  // In dark mode, --bg-card should be dark
  const cardVarDark = isDark(bgCard);
  if (cardVarDark === true) pass(`dark mode --bg-card is dark: "${bgCard}"`);
  else if (cardVarDark === false) fail(`dark mode --bg-card appears LIGHT (regression!): "${bgCard}"`);
  else console.log(`  WARN: dark --bg-card could not be parsed: "${bgCard}"`);

  // .card/.w element should be dark
  if (cardBg !== null) {
    const elDark = isDark(cardBg);
    if (elDark === true) pass(`dark mode .card/.w bg is dark: "${cardBg}"`);
    else if (elDark === false) fail(`dark mode .card/.w bg appears LIGHT (regression!): "${cardBg}"`);
    else console.log(`  WARN: dark .card/.w bg could not be parsed: "${cardBg}"`);
  } else {
    console.log("  WARN: .card/.w not found on /dashboard in dark mode");
  }

  await page.screenshot({ path: path.join(SHOTS_DIR, "gx14_verify_dark_regression.png"), fullPage: true });
  console.log("  Screenshot: gx14_verify_dark_regression.png");

  // Check /transactions thead th in dark
  await page.goto(BASE + "/transactions");
  await ready(page);
  // theme pref persists in localStorage — reload to apply
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "dark",
    { timeout: 10_000 }
  );
  const txnThBg = await getElBg(page, ".txn-table thead th");
  if (txnThBg !== null) {
    const elDark = isDark(txnThBg);
    if (elDark === true) pass(`dark mode /transactions thead th bg is dark: "${txnThBg}"`);
    else if (elDark === false) fail(`dark mode /transactions thead th appears LIGHT (regression!): "${txnThBg}"`);
    else console.log(`  WARN: dark thead th bg could not be parsed: "${txnThBg}"`);
  } else {
    console.log("  WARN: /transactions .txn-table thead th not found in dark mode");
  }

  await page.close();
}

// ─── Finish ──────────────────────────────────────────────────────────────────

await browser.close();

console.log(`\n=== RESULT: ${failures === 0 ? "ALL CHECKS PASSED" : failures + " FAILURE(S)"} ===`);
process.exitCode = failures > 0 ? 1 : 0;
