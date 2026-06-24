// GX13 verification: /budgets light-mode CSS fixes for category-name buttons,
// progress-bar tracks, and stat tiles. Also spot-checks dark mode for regression.
// Exits non-zero on any FAIL.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
let failed = false;
const fail = (m) => { console.error("FAIL: " + m); failed = true; };
const pass = (m) => console.log("PASS: " + m);

function isLight(rgb, threshold = 200) {
  // Returns true if all channels >= threshold (i.e. light/white-ish)
  const m = rgb.match(/\d+/g);
  if (!m) return false;
  return m.slice(0, 3).every((v) => parseInt(v) >= threshold);
}

function isDark(rgb, threshold = 60) {
  const m = rgb.match(/\d+/g);
  if (!m) return false;
  return m.slice(0, 3).every((v) => parseInt(v) <= threshold);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e));

  // ── LIGHT MODE ─────────────────────────────────────────────────────────────
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });

  // Set light theme via localStorage and reload
  await page.evaluate(() =>
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }))
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "light",
    { timeout: 10000 }
  );
  await page.waitForTimeout(500);

  await page.screenshot({ path: "e2e/gx13_verify_budgets_light.png", fullPage: true });

  // 1) .budget-drill / button.row-desc color — expect dark ≈ rgb(28,28,30)
  const drillColors = await page.evaluate(() => {
    const els = [
      ...document.querySelectorAll(".budget-drill"),
      ...document.querySelectorAll("button.row-desc"),
    ].slice(0, 3);
    return els.map((el) => ({
      tag: el.tagName + (el.className ? "." + [...el.classList].join(".") : ""),
      color: getComputedStyle(el).color,
    }));
  });

  if (drillColors.length === 0) {
    fail("No .budget-drill or button.row-desc elements found on /budgets");
  } else {
    for (const { tag, color } of drillColors) {
      console.log(`  [light] ${tag} color = ${color}`);
      // Near-white would be rgb(244,244,245) or similar — channels all >200
      if (isLight(color, 200)) {
        fail(`Category name button color is near-white in light mode: ${color}`);
      } else {
        pass(`Category name button dark in light mode: ${color}`);
      }
    }
  }

  // 2) .bar (progress-bar track) backgroundColor — expect light ≈ rgb(239,237,232)
  const barBgs = await page.evaluate(() => {
    const els = [...document.querySelectorAll(".bar")].slice(0, 3);
    return els.map((el) => ({
      tag: el.tagName + (el.className ? "." + [...el.classList].join(".") : ""),
      bg: getComputedStyle(el).backgroundColor,
    }));
  });

  if (barBgs.length === 0) {
    fail("No .bar elements found on /budgets");
  } else {
    for (const { tag, bg } of barBgs) {
      console.log(`  [light] ${tag} backgroundColor = ${bg}`);
      // Dark background would be rgb(32,32,34) — channels all <60
      if (isDark(bg, 60)) {
        fail(`Progress-bar track is dark in light mode: ${bg}`);
      } else {
        pass(`Progress-bar track is light in light mode: ${bg}`);
      }
    }
  }

  // 3) .stat (stat tile) backgroundColor — expect white rgb(255,255,255)
  const statBgs = await page.evaluate(() => {
    const els = [...document.querySelectorAll(".stat")].slice(0, 3);
    return els.map((el) => ({
      tag: el.tagName + (el.className ? "." + [...el.classList].join(".") : ""),
      bg: getComputedStyle(el).backgroundColor,
    }));
  });

  if (statBgs.length === 0) {
    fail("No .stat elements found on /budgets");
  } else {
    for (const { tag, bg } of statBgs) {
      console.log(`  [light] ${tag} backgroundColor = ${bg}`);
      if (isDark(bg, 60)) {
        fail(`Stat tile is dark in light mode: ${bg}`);
      } else {
        pass(`Stat tile is light/white in light mode: ${bg}`);
      }
    }
  }

  // ── DARK MODE spot-check ────────────────────────────────────────────────────
  await page.evaluate(() =>
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark" }))
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "dark",
    { timeout: 10000 }
  );
  await page.waitForTimeout(500);

  await page.screenshot({ path: "e2e/gx13_verify_budgets_dark.png", fullPage: true });

  // .bar dark mode — expect dark background
  const darkBarBgs = await page.evaluate(() => {
    const els = [...document.querySelectorAll(".bar")].slice(0, 3);
    return els.map((el) => getComputedStyle(el).backgroundColor);
  });
  for (const bg of darkBarBgs) {
    console.log(`  [dark] .bar backgroundColor = ${bg}`);
    if (isLight(bg, 200)) {
      fail(`Progress-bar track is light in dark mode (regression): ${bg}`);
    } else {
      pass(`Progress-bar track is dark in dark mode: ${bg}`);
    }
  }

  // .stat dark mode — expect dark background
  const darkStatBgs = await page.evaluate(() => {
    const els = [...document.querySelectorAll(".stat")].slice(0, 3);
    return els.map((el) => getComputedStyle(el).backgroundColor);
  });
  for (const bg of darkStatBgs) {
    console.log(`  [dark] .stat backgroundColor = ${bg}`);
    if (isLight(bg, 200)) {
      fail(`Stat tile is light in dark mode (regression): ${bg}`);
    } else {
      pass(`Stat tile is dark in dark mode: ${bg}`);
    }
  }

  // category names in dark mode — expect light (readable)
  const darkDrillColors = await page.evaluate(() => {
    const els = [
      ...document.querySelectorAll(".budget-drill"),
      ...document.querySelectorAll("button.row-desc"),
    ].slice(0, 3);
    return els.map((el) => getComputedStyle(el).color);
  });
  for (const color of darkDrillColors) {
    console.log(`  [dark] .budget-drill color = ${color}`);
    if (isDark(color, 60)) {
      fail(`Category name button is near-black in dark mode (regression): ${color}`);
    } else {
      pass(`Category name button is light in dark mode: ${color}`);
    }
  }

} finally {
  await browser.close();
  if (failed) process.exitCode = 1;
}
