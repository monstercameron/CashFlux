// GX7-F1 verification — ultra-wide cap at min-width:1441px.
//
// Checks:
//  1. 2560×1440 /dashboard + /transactions: main.width ≈ 1440, no h-overflow, centered.
//  2. 1280×900  /dashboard (regression): media query must NOT apply; full width unchanged.
//  3. 1440×900  /dashboard (gate-boundary): 1440 < 1441 breakpoint, must be unchanged.
//
// Screenshots saved to e2e/screenshots/.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
import { ready } from "./_ready.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS_DIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SS_DIR)) fs.mkdirSync(SS_DIR, { recursive: true });

let passed = 0;
let failed = 0;

const pass = (m) => { console.log(`  PASS  ${m}`); passed++; };
const fail = (m) => { console.error(`  FAIL  ${m}`); failed++; };

// Measure main geometry in a page already at the right URL.
async function measure(page) {
  return page.evaluate(() => {
    const main = document.querySelector("main");
    if (!main) return null;
    const rect = main.getBoundingClientRect();
    const scrollW = document.documentElement.scrollWidth;
    const clientW = document.documentElement.clientWidth;
    // Rail: first <nav> sibling of <main>, or assume 64px fallback.
    const nav = document.querySelector("nav");
    const railW = nav ? nav.getBoundingClientRect().width : 64;
    return {
      mainWidth: rect.width,
      mainLeft: rect.left,
      mainRight: rect.right,
      scrollWidth: scrollW,
      clientWidth: clientW,
      hOverflow: scrollW > clientW,
      railWidth: railW,
      viewportWidth: window.innerWidth,
    };
  });
}

const browser = await chromium.launch({ headless: true });

// ---------------------------------------------------------------------------
// 1a. 2560×1440 — /dashboard
// ---------------------------------------------------------------------------
console.log("\n── Viewport 2560×1440  /dashboard ──");
{
  const ctx = await browser.newContext({ viewport: { width: 2560, height: 1440 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/dashboard`);
  await ready(page);
  const m = await measure(page);
  console.log("  measured:", JSON.stringify(m));

  const ssPath = path.join(SS_DIR, "gx7_verify_2560_dashboard.png");
  await page.screenshot({ path: ssPath, fullPage: false });
  console.log(`  screenshot → ${ssPath}`);

  if (m === null) {
    fail("no <main> element found");
  } else {
    // Width cap: should be ≈1440 (allow ±20px for padding/border-box variance)
    if (Math.abs(m.mainWidth - 1440) <= 20) {
      pass(`main.width=${m.mainWidth.toFixed(1)} ≈ 1440`);
    } else {
      fail(`main.width=${m.mainWidth.toFixed(1)} — expected ≈1440 (not capped or wrong value)`);
    }
    // No horizontal overflow
    if (!m.hOverflow) {
      pass(`no h-overflow (scrollWidth=${m.scrollWidth} clientWidth=${m.clientWidth})`);
    } else {
      fail(`h-overflow detected (scrollWidth=${m.scrollWidth} > clientWidth=${m.clientWidth})`);
    }
    // Centered: dead space each side should be roughly equal; left offset > rail width
    const leftDeadSpace = m.mainLeft;
    const rightDeadSpace = m.clientWidth - m.mainRight;
    if (m.mainLeft > m.railWidth) {
      pass(`main left offset (${m.mainLeft.toFixed(1)}) > rail width (${m.railWidth.toFixed(1)})`);
    } else {
      fail(`main left offset (${m.mainLeft.toFixed(1)}) ≤ rail width (${m.railWidth.toFixed(1)}) — not centered`);
    }
    // Rough symmetry: left dead-space minus right dead-space ≤ 20% of client width
    const asymmetry = Math.abs(leftDeadSpace - rightDeadSpace);
    if (asymmetry <= m.clientWidth * 0.20) {
      pass(`roughly centered: leftDead=${leftDeadSpace.toFixed(1)} rightDead=${rightDeadSpace.toFixed(1)} asymmetry=${asymmetry.toFixed(1)}`);
    } else {
      fail(`not centered: leftDead=${leftDeadSpace.toFixed(1)} rightDead=${rightDeadSpace.toFixed(1)} asymmetry=${asymmetry.toFixed(1)}`);
    }
  }
  await ctx.close();
}

// ---------------------------------------------------------------------------
// 1b. 2560×1440 — /transactions
// ---------------------------------------------------------------------------
console.log("\n── Viewport 2560×1440  /transactions ──");
{
  const ctx = await browser.newContext({ viewport: { width: 2560, height: 1440 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/transactions`);
  await ready(page);
  const m = await measure(page);
  console.log("  measured:", JSON.stringify(m));

  const ssPath = path.join(SS_DIR, "gx7_verify_2560_transactions.png");
  await page.screenshot({ path: ssPath, fullPage: false });
  console.log(`  screenshot → ${ssPath}`);

  if (m === null) {
    fail("no <main> element found");
  } else {
    if (Math.abs(m.mainWidth - 1440) <= 20) {
      pass(`main.width=${m.mainWidth.toFixed(1)} ≈ 1440`);
    } else {
      fail(`main.width=${m.mainWidth.toFixed(1)} — expected ≈1440`);
    }
    if (!m.hOverflow) {
      pass(`no h-overflow (scrollWidth=${m.scrollWidth} clientWidth=${m.clientWidth})`);
    } else {
      fail(`h-overflow detected (scrollWidth=${m.scrollWidth} > clientWidth=${m.clientWidth})`);
    }
    if (m.mainLeft > m.railWidth) {
      pass(`main left offset (${m.mainLeft.toFixed(1)}) > rail (${m.railWidth.toFixed(1)})`);
    } else {
      fail(`main left offset (${m.mainLeft.toFixed(1)}) ≤ rail (${m.railWidth.toFixed(1)}) — not centered`);
    }
    const leftDeadSpace = m.mainLeft;
    const rightDeadSpace = m.clientWidth - m.mainRight;
    const asymmetry = Math.abs(leftDeadSpace - rightDeadSpace);
    if (asymmetry <= m.clientWidth * 0.20) {
      pass(`roughly centered: leftDead=${leftDeadSpace.toFixed(1)} rightDead=${rightDeadSpace.toFixed(1)} asymmetry=${asymmetry.toFixed(1)}`);
    } else {
      fail(`not centered: leftDead=${leftDeadSpace.toFixed(1)} rightDead=${rightDeadSpace.toFixed(1)} asymmetry=${asymmetry.toFixed(1)}`);
    }
  }
  await ctx.close();
}

// ---------------------------------------------------------------------------
// 2. 1280×900 — regression: media query must NOT apply
// ---------------------------------------------------------------------------
console.log("\n── Viewport 1280×900  /dashboard (regression) ──");
{
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/dashboard`);
  await ready(page);
  const m = await measure(page);
  console.log("  measured:", JSON.stringify(m));

  const ssPath = path.join(SS_DIR, "gx7_verify_1280.png");
  await page.screenshot({ path: ssPath, fullPage: false });
  console.log(`  screenshot → ${ssPath}`);

  if (m === null) {
    fail("no <main> element found");
  } else {
    // Media query should NOT apply: main should be full width minus the rail.
    // Expected: mainWidth > 1100 (definitely not capped at 1440 on a 1280 viewport).
    // The cap at 1440 is irrelevant here anyway (viewport < 1441), so main should fill
    // available space (≥ viewport - rail - scrollbar).
    const availableApprox = m.clientWidth - m.railWidth;
    if (m.mainWidth >= availableApprox * 0.80) {
      pass(`main.width=${m.mainWidth.toFixed(1)} fills available space (avail≈${availableApprox.toFixed(1)}) — cap not applied`);
    } else {
      fail(`main.width=${m.mainWidth.toFixed(1)} unexpectedly narrow vs available (${availableApprox.toFixed(1)}) — cap may have fired`);
    }
    // No overflow
    if (!m.hOverflow) {
      pass(`no h-overflow (scrollWidth=${m.scrollWidth} clientWidth=${m.clientWidth})`);
    } else {
      fail(`h-overflow detected (scrollWidth=${m.scrollWidth} > clientWidth=${m.clientWidth})`);
    }
  }
  await ctx.close();
}

// ---------------------------------------------------------------------------
// 3. 1440×900 — gate boundary: 1440 < 1441 → unchanged
// ---------------------------------------------------------------------------
console.log("\n── Viewport 1440×900  /dashboard (gate boundary) ──");
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/dashboard`);
  await ready(page);
  const m = await measure(page);
  console.log("  measured:", JSON.stringify(m));

  const ssPath = path.join(SS_DIR, "gx7_verify_1440.png");
  await page.screenshot({ path: ssPath, fullPage: false });
  console.log(`  screenshot → ${ssPath}`);

  if (m === null) {
    fail("no <main> element found");
  } else {
    // At 1440px viewport the breakpoint (min-width:1441px) is NOT active.
    // main should fill available space, not be capped / centered.
    const availableApprox = m.clientWidth - m.railWidth;
    if (m.mainWidth >= availableApprox * 0.80) {
      pass(`main.width=${m.mainWidth.toFixed(1)} fills available space (avail≈${availableApprox.toFixed(1)}) — breakpoint correctly inactive at 1440`);
    } else {
      fail(`main.width=${m.mainWidth.toFixed(1)} unexpectedly narrow — breakpoint may have fired at 1440 (should require ≥1441)`);
    }
    // No overflow
    if (!m.hOverflow) {
      pass(`no h-overflow (scrollWidth=${m.scrollWidth} clientWidth=${m.clientWidth})`);
    } else {
      fail(`h-overflow detected (scrollWidth=${m.scrollWidth} > clientWidth=${m.clientWidth})`);
    }
  }
  await ctx.close();
}

// ---------------------------------------------------------------------------
await browser.close();

console.log(`\n══ GX7-F1 result: ${passed} passed, ${failed} failed ══`);
if (failed > 0) process.exit(1);
