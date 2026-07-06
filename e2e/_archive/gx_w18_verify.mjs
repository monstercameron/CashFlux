// W-18 runtime verification: AreaChart draw-in (pathLength="1" + wonder-chart-line/area).
// Checks: animation wiring, settled line integrity (critical), off/reduced-motion states,
// no console errors, chart not distorted.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOT_DIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOT_DIR)) fs.mkdirSync(SHOT_DIR, { recursive: true });

let passed = 0;
let failed = 0;
const results = [];

function pass(label, detail = "") {
  passed++;
  results.push({ ok: true, label, detail });
  console.log(`  PASS  ${label}${detail ? " — " + detail : ""}`);
}
function fail(label, detail = "") {
  failed++;
  results.push({ ok: false, label, detail });
  console.error(`  FAIL  ${label}${detail ? " — " + detail : ""}`);
}
function info(msg) {
  console.log(`        ${msg}`);
}

// Navigate to dashboard and wait for a wonder-chart-line to appear.
async function gotoChart(page, wonderOff = false, reduceMotion = false) {
  const url = BASE + "/dashboard";
  await page.goto(url, { waitUntil: "domcontentloaded" });
  // Wait for the app to boot (wasm ready indicator or the chart itself).
  await page.waitForSelector(".wonder-chart-line, svg path", { timeout: 30000 }).catch(() => {});
  // Also try /reports if dashboard has no chart.
  const hasChart = await page.$(".wonder-chart-line");
  if (!hasChart) {
    await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
    await page.waitForSelector(".wonder-chart-line, svg path", { timeout: 20000 }).catch(() => {});
  }
}

const browser = await chromium.launch({ headless: true });

// ── CHECK 1: DEFAULT — animation wiring + pathLength ──────────────────────────
console.log("\n[1] DEFAULT: animation wiring + pathLength");
{
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const consoleErrors = [];
  page.on("pageerror", (e) => consoleErrors.push(String(e)));
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });

  await gotoChart(page);

  const lineSel = ".wonder-chart-line";
  const lineEl = await page.$(lineSel);

  if (!lineEl) {
    fail("wonder-chart-line exists", "element NOT found in DOM");
  } else {
    pass("wonder-chart-line exists");

    // pathLength attribute
    const pl = await lineEl.getAttribute("pathLength");
    if (pl === "1") pass('pathLength="1"', `got "${pl}"`);
    else fail('pathLength="1"', `got "${pl}"`);

    // animation-name
    const animName = await page.evaluate(() => {
      const el = document.querySelector(".wonder-chart-line");
      return el ? getComputedStyle(el).animationName : "NOT_FOUND";
    });
    if (animName === "wonder-chart-draw") pass("animation-name = wonder-chart-draw", animName);
    else fail("animation-name = wonder-chart-draw", `got "${animName}"`);

    // area fill
    const areaEl = await page.$(".wonder-chart-area");
    if (areaEl) pass("wonder-chart-area exists");
    else fail("wonder-chart-area exists", "not found");
  }

  // Screenshot
  await page.screenshot({ path: path.join(SHOT_DIR, "w18_chart_default.png"), fullPage: false });
  info("screenshot: w18_chart_default.png");

  if (consoleErrors.length > 0) fail("no console errors (default)", consoleErrors.join("; "));
  else pass("no console errors (default)");

  await ctx.close();
}

// ── CHECK 2: SETTLED LINE INTEGRITY (critical) ─────────────────────────────────
console.log("\n[2] SETTLED LINE INTEGRITY (critical — dashoffset should be 0 after animation)");
{
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const consoleErrors = [];
  page.on("pageerror", (e) => consoleErrors.push(String(e)));

  await gotoChart(page);

  // Wait for animation to settle (wonder-dur-slow ~300ms; give 800ms for safety).
  await page.waitForTimeout(800);

  const settled = await page.evaluate(() => {
    const el = document.querySelector(".wonder-chart-line");
    if (!el) return null;
    const cs = getComputedStyle(el);
    return {
      strokeDashoffset: cs.strokeDashoffset,
      strokeDasharray: cs.strokeDasharray,
      animationName: cs.animationName,
      animationFillMode: cs.animationFillMode,
      opacity: cs.opacity,
      display: cs.display,
      visibility: cs.visibility,
    };
  });

  if (!settled) {
    fail("settled: .wonder-chart-line found", "element missing");
  } else {
    info(`settled strokeDashoffset = "${settled.strokeDashoffset}"`);
    info(`settled strokeDasharray  = "${settled.strokeDasharray}"`);
    info(`settled animationName    = "${settled.animationName}"`);
    info(`settled animationFillMode= "${settled.animationFillMode}"`);
    info(`settled opacity          = "${settled.opacity}"`);

    // dashoffset 0 means the full path is drawn.
    const dashoffset = parseFloat(settled.strokeDashoffset);
    if (Math.abs(dashoffset) < 0.01) {
      pass("settled stroke-dashoffset = 0 (full line drawn)", `got "${settled.strokeDashoffset}"`);
    } else {
      fail("settled stroke-dashoffset = 0 (full line drawn)", `got "${settled.strokeDashoffset}" — line may be dashed/invisible`);
    }

    // Check fill-mode is "both" so the final state sticks.
    if (settled.animationFillMode === "both") pass("animation-fill-mode = both", settled.animationFillMode);
    else fail("animation-fill-mode = both", `got "${settled.animationFillMode}"`);

    // Check visibility.
    if (settled.opacity !== "0" && settled.visibility !== "hidden" && settled.display !== "none") {
      pass("line is visible (opacity/visibility/display)", `opacity=${settled.opacity} visibility=${settled.visibility}`);
    } else {
      fail("line is visible", `opacity=${settled.opacity} visibility=${settled.visibility} display=${settled.display}`);
    }
  }

  // Screenshot settled state.
  await page.screenshot({ path: path.join(SHOT_DIR, "w18_chart_settled.png"), fullPage: false });
  info("screenshot: w18_chart_settled.png");

  // Visual description logged for human review.
  // We check that the SVG path d attribute is non-trivial (has M and L/C commands).
  const pathD = await page.evaluate(() => {
    const el = document.querySelector(".wonder-chart-line");
    return el ? el.getAttribute("d") : null;
  });
  if (pathD && pathD.length > 10 && /[LC]/.test(pathD)) {
    pass("chart line path has shape data (not degenerate)", `d starts: ${pathD.substring(0, 40)}…`);
  } else if (pathD) {
    info(`line path d = "${pathD.substring(0, 80)}"`);
  }

  if (consoleErrors.length > 0) fail("no console errors (settled)", consoleErrors.join("; "));
  else pass("no console errors (settled)");

  await ctx.close();
}

// ── CHECK 3: OFF state (data-wonder="off") ────────────────────────────────────
console.log('\n[3] OFF STATE: data-wonder="off"');
{
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const consoleErrors = [];
  page.on("pageerror", (e) => consoleErrors.push(String(e)));

  await gotoChart(page);

  // Set data-wonder="off" on root <html>.
  await page.evaluate(() => {
    document.documentElement.setAttribute("data-wonder", "off");
  });
  await page.waitForTimeout(200);

  const off = await page.evaluate(() => {
    const el = document.querySelector(".wonder-chart-line");
    if (!el) return null;
    const cs = getComputedStyle(el);
    return {
      animationName: cs.animationName,
      strokeDashoffset: cs.strokeDashoffset,
      strokeDasharray: cs.strokeDasharray,
      opacity: cs.opacity,
    };
  });

  if (!off) {
    fail("off: .wonder-chart-line found", "element missing");
  } else {
    info(`off animationName       = "${off.animationName}"`);
    info(`off strokeDashoffset    = "${off.strokeDashoffset}"`);
    info(`off strokeDasharray     = "${off.strokeDasharray}"`);

    if (off.animationName === "none") pass("off: animation-name = none", off.animationName);
    else fail("off: animation-name = none", `got "${off.animationName}"`);

    const dashoffset = parseFloat(off.strokeDashoffset);
    if (Math.abs(dashoffset) < 0.01) {
      pass("off: stroke-dashoffset = 0 (line immediately visible)", `got "${off.strokeDashoffset}"`);
    } else {
      fail("off: stroke-dashoffset = 0 (line immediately visible)", `got "${off.strokeDashoffset}" — line dashed/invisible with wonder off`);
    }

    if (off.opacity !== "0") pass("off: line visible (opacity not 0)", `opacity=${off.opacity}`);
    else fail("off: line visible", `opacity=${off.opacity}`);
  }

  await page.screenshot({ path: path.join(SHOT_DIR, "w18_chart_off.png"), fullPage: false });
  info("screenshot: w18_chart_off.png");

  if (consoleErrors.length > 0) fail("no console errors (off)", consoleErrors.join("; "));
  else pass("no console errors (off)");

  await ctx.close();
}

// ── CHECK 4: REDUCED MOTION ───────────────────────────────────────────────────
console.log("\n[4] REDUCED MOTION: prefers-reduced-motion: reduce");
{
  const ctx = await browser.newContext({ reducedMotion: "reduce" });
  const page = await ctx.newPage();
  const consoleErrors = [];
  page.on("pageerror", (e) => consoleErrors.push(String(e)));

  await gotoChart(page);
  await page.waitForTimeout(200);

  const rm = await page.evaluate(() => {
    const el = document.querySelector(".wonder-chart-line");
    if (!el) return null;
    const cs = getComputedStyle(el);
    return {
      animationName: cs.animationName,
      strokeDashoffset: cs.strokeDashoffset,
      opacity: cs.opacity,
    };
  });

  if (!rm) {
    fail("reduced-motion: .wonder-chart-line found", "element missing");
  } else {
    info(`reduced-motion animationName    = "${rm.animationName}"`);
    info(`reduced-motion strokeDashoffset = "${rm.strokeDashoffset}"`);

    if (rm.animationName === "none") pass("reduced-motion: animation-name = none", rm.animationName);
    else fail("reduced-motion: animation-name = none", `got "${rm.animationName}"`);

    const dashoffset = parseFloat(rm.strokeDashoffset);
    if (Math.abs(dashoffset) < 0.01) {
      pass("reduced-motion: stroke-dashoffset = 0 (line fully visible)", `got "${rm.strokeDashoffset}"`);
    } else {
      fail("reduced-motion: stroke-dashoffset = 0 (line fully visible)", `got "${rm.strokeDashoffset}" — line hidden under reduced motion`);
    }

    if (rm.opacity !== "0") pass("reduced-motion: line visible", `opacity=${rm.opacity}`);
    else fail("reduced-motion: line visible", `opacity=${rm.opacity}`);
  }

  await page.screenshot({ path: path.join(SHOT_DIR, "w18_chart_reduced_motion.png"), fullPage: false });
  info("screenshot: w18_chart_reduced_motion.png");

  if (consoleErrors.length > 0) fail("no console errors (reduced-motion)", consoleErrors.join("; "));
  else pass("no console errors (reduced-motion)");

  await ctx.close();
}

// ── CHECK 5: CHART SHAPE / NO DISTORTION ────────────────────────────────────
console.log("\n[5] CHART SHAPE: area fill + no animation artifacts");
{
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const consoleErrors = [];
  page.on("pageerror", (e) => consoleErrors.push(String(e)));

  await gotoChart(page);
  await page.waitForTimeout(800);

  const shape = await page.evaluate(() => {
    const line = document.querySelector(".wonder-chart-line");
    const area = document.querySelector(".wonder-chart-area");
    if (!line || !area) return null;
    const lineBox = line.getBoundingClientRect();
    const areaBox = area.getBoundingClientRect();
    const lineCS = getComputedStyle(line);
    const areaCS = getComputedStyle(area);
    return {
      lineWidth: lineBox.width,
      lineHeight: lineBox.height,
      areaWidth: areaBox.width,
      areaHeight: areaBox.height,
      areaOpacity: areaCS.opacity,
      areaAnimName: areaCS.animationName,
      lineFill: lineCS.fill,
      lineStroke: lineCS.stroke,
    };
  });

  if (!shape) {
    fail("chart shape: both line and area elements found", "one or both missing");
  } else {
    info(`line bounding box: ${shape.lineWidth.toFixed(1)} × ${shape.lineHeight.toFixed(1)}`);
    info(`area bounding box: ${shape.areaWidth.toFixed(1)} × ${shape.areaHeight.toFixed(1)}`);
    info(`area opacity: ${shape.areaOpacity}, area animation: ${shape.areaAnimName}`);
    info(`line fill: ${shape.lineFill}, line stroke: ${shape.lineStroke}`);

    if (shape.lineWidth > 10 && shape.lineHeight > 2) pass("line has non-degenerate bounding box", `${shape.lineWidth.toFixed(0)}×${shape.lineHeight.toFixed(0)}`);
    else fail("line has non-degenerate bounding box", `got ${shape.lineWidth.toFixed(1)}×${shape.lineHeight.toFixed(1)}`);

    if (shape.areaWidth > 10) pass("area has non-degenerate width", `${shape.areaWidth.toFixed(0)}px`);
    else fail("area has non-degenerate width", `got ${shape.areaWidth.toFixed(1)}`);

    if (parseFloat(shape.areaOpacity) > 0) pass("area is visible (opacity > 0)", `opacity=${shape.areaOpacity}`);
    else fail("area is visible", `opacity=${shape.areaOpacity}`);

    // After settling, area animation should have played (opacity 1 via both fill-mode).
    if (shape.areaAnimName === "wonder-chart-fade") pass("area animation-name = wonder-chart-fade", shape.areaAnimName);
    else fail("area animation-name = wonder-chart-fade", `got "${shape.areaAnimName}"`);
  }

  if (consoleErrors.length > 0) fail("no console errors (shape check)", consoleErrors.join("; "));
  else pass("no console errors (shape check)");

  await ctx.close();
}

await browser.close();

// ── SUMMARY ──────────────────────────────────────────────────────────────────
console.log(`\n${"─".repeat(60)}`);
console.log(`W-18 VERIFICATION SUMMARY: ${passed} passed, ${failed} failed`);
console.log("─".repeat(60));
results.forEach(({ ok, label, detail }) => {
  console.log(`  ${ok ? "PASS" : "FAIL"}  ${label}${detail ? " — " + detail : ""}`);
});
console.log("─".repeat(60));

if (failed > 0) {
  console.error(`\nW-18 RESULT: FAIL (${failed} check(s) failed)`);
  process.exitCode = 1;
} else {
  console.log(`\nW-18 RESULT: PASS — all checks passed`);
}
