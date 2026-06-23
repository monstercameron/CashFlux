// GLAMOR G4 — Budgets visual review ("The Mid-Month Pulse" / Renu).
// Captures screenshots at 1280/1440/768 × dark + light themes.
// Saves to e2e/screenshots/ with names glamor_04_budgets_<width>_<theme>.png.
// Also captures full-page shot at 1280 dark and DOM info.
// Light-theme recipe: inject cashflux:prefs with theme:'light' + reload, then
// poll for data-theme === 'light' on <html>. Confirmed working per task brief.
// Not a pass/fail gate — purely a visual evidence harvest.
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

const WIDTHS = [1280, 1440, 768];

// bootWithTheme: boots WASM, injects theme via cashflux:prefs (the solved recipe),
// reloads, polls for data-theme, then navigates to /budgets.
async function bootWithTheme(browser, width, theme) {
  const ctx = await browser.newContext({ viewport: { width, height: 900 } });
  const page = await ctx.newPage();

  // Initial load — let WASM boot and write its own prefs first.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(
    'nav[aria-label="Main navigation"] a[title], #app .bento, #app .w',
    { timeout: 60000 }
  );
  // Brief pause to let WASM finish writing localStorage.
  await page.waitForTimeout(800);

  // SOLVED LIGHT-THEME RECIPE: patch cashflux:prefs with the chosen theme and reload.
  // This overwrites whatever WASM wrote, so the next boot reads our value.
  await page.evaluate((t) => {
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      const p = raw ? JSON.parse(raw) : {};
      p.theme = t;
      localStorage.setItem("cashflux:prefs", JSON.stringify(p));
    } catch (_) {
      localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: t }));
    }
  }, theme);

  // Reload so the new prefs take effect on boot.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(
    'nav[aria-label="Main navigation"] a[title], #app .bento, #app .w',
    { timeout: 60000 }
  );

  // Poll for data-theme on <html> — confirm theme applied.
  let themeConfirmed = false;
  for (let i = 0; i < 40; i++) {
    const actual = await page.evaluate(
      () => document.documentElement.getAttribute("data-theme")
    );
    if (theme === "light" && actual === "light") { themeConfirmed = true; break; }
    if (theme === "dark" && (actual === "dark" || actual === null)) { themeConfirmed = true; break; }
    await page.waitForTimeout(200);
  }
  if (!themeConfirmed) {
    console.warn(`  [warn] theme '${theme}' not confirmed on <html> data-theme — proceeding.`);
  } else {
    console.log(`  [ok] theme '${theme}' confirmed on <html>.`);
  }

  // Navigate to /budgets via the nav link.
  try {
    await page.locator('nav a[title="Budgets"]').first().click();
    await page.waitForTimeout(800);
  } catch (_) {
    try {
      await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(800);
    } catch (_2) {}
  }

  // Final settle — give any animations or data loads a moment.
  await page.waitForTimeout(600);
  return { page, ctx };
}

const browser = await chromium.launch({ headless: true });

try {
  for (const theme of ["dark", "light"]) {
    for (const width of WIDTHS) {
      const errors = [];
      console.log(`Capturing ${width}px / ${theme}…`);
      const { page, ctx } = await bootWithTheme(browser, width, theme);
      page.on("pageerror", (e) => errors.push(String(e)));

      const shotPath = path.join(
        SHOTS_DIR,
        `glamor_04_budgets_${width}_${theme}.png`
      );
      await page.screenshot({ path: shotPath, fullPage: false });
      console.log(`  wrote ${path.basename(shotPath)}`);

      // Full-page shot at 1280 dark.
      if (width === 1280 && theme === "dark") {
        const fullPath = path.join(
          SHOTS_DIR,
          `glamor_04_budgets_${width}_${theme}_full.png`
        );
        await page.screenshot({ path: fullPath, fullPage: true });
        console.log(`  wrote ${path.basename(fullPath)}`);

        // DOM audit: harvest structure, colors, row metrics, sub-line text.
        const domInfo = await page.evaluate(() => {
          const confirmed = document.documentElement.getAttribute("data-theme");

          // Headings and section structure.
          const headings = Array.from(
            document.querySelectorAll("h1, h2, h3, .card-title, [class*='heading']")
          ).map((el) => el.innerText.trim()).filter(Boolean);

          // Summary stat tiles (SPENT / BUDGETED / LEFT).
          const statTiles = Array.from(
            document.querySelectorAll(".stat-grid .stat, .stat-grid > *")
          ).map((el) => el.innerText.trim()).filter(Boolean);

          // Budget row elements: collect each .budget div.
          const budgetRows = Array.from(
            document.querySelectorAll(".budget")
          ).map((row) => {
            const head = row.querySelector(".budget-head");
            const bar = row.querySelector(".bar");
            const fill = row.querySelector("[class*='bar-fill']");
            const subLines = Array.from(row.querySelectorAll(".budget-sub")).map(
              (el) => el.innerText.trim()
            );
            const buttons = Array.from(row.querySelectorAll("button")).map(
              (b) => b.innerText.trim() || b.getAttribute("aria-label") || b.title
            );

            // Colors & widths.
            const fillStyle = fill ? fill.getAttribute("style") : "";
            const fillClass = fill ? fill.className : "";

            // Row height.
            const rect = row.getBoundingClientRect();

            return {
              headText: head ? head.innerText.trim() : "",
              fillWidth: fillStyle,
              fillClass,
              subLines,
              buttons,
              rowHeight: Math.round(rect.height),
            };
          });

          // Status summary pills (over budget / near limit).
          const pills = Array.from(
            document.querySelectorAll(".pill")
          ).map((p) => p.innerText.trim());

          // Over/near count from page copy.
          const budgetSub = Array.from(
            document.querySelectorAll(".budget-sub")
          ).map((el) => el.innerText.trim()).filter(Boolean);

          // Period control presence.
          const periodControl = document.querySelector(".reso-control, [class*='reso']");
          const periodText = periodControl ? periodControl.innerText.trim() : "(not found)";

          // Add-budget form / button.
          const addBtns = Array.from(document.querySelectorAll("button"))
            .filter((b) => {
              const t = (b.innerText + b.getAttribute("aria-label") + b.title).toLowerCase();
              return t.includes("add") || t.includes("budget");
            })
            .map((b) => b.innerText.trim() || b.getAttribute("aria-label"));

          // Assign banner (zero-based methodology).
          const assignBanner = document.querySelector(".budget-sub");
          const assignText = assignBanner ? assignBanner.innerText.trim() : "(none)";

          // Check for hidden labels (form-grid labels vs. visual labels).
          const formLabels = Array.from(document.querySelectorAll("label")).map(
            (l) => l.innerText.trim()
          ).filter(Boolean);

          // Check bar ARIA attributes.
          const bars = Array.from(document.querySelectorAll('[role="progressbar"]')).map(
            (b) => ({
              valuenow: b.getAttribute("aria-valuenow"),
              label: b.getAttribute("aria-label"),
              fillClass: b.firstElementChild ? b.firstElementChild.className : "",
            })
          );

          // Check contrast of sub-lines by computed color.
          const subSample = document.querySelector(".budget-sub");
          const subColor = subSample
            ? getComputedStyle(subSample).color
            : "(none)";

          // Check ordering: are over-budget rows near the top?
          const overRows = Array.from(document.querySelectorAll(".budget"))
            .map((row, idx) => {
              const fill = row.querySelector("[class*='bar-fill']");
              const isOver = fill && fill.className.includes("over");
              const isNear = fill && fill.className.includes("near");
              const headEl = row.querySelector(".budget-head");
              return {
                idx,
                name: headEl ? headEl.innerText.trim().slice(0, 40) : "",
                isOver,
                isNear,
              };
            });

          // Check visible rows above the fold.
          const allBudgetDivs = Array.from(document.querySelectorAll(".budget"));
          const aboveFold = allBudgetDivs.filter((r) => {
            const rect = r.getBoundingClientRect();
            return rect.top < window.innerHeight && rect.bottom > 0;
          }).length;

          return {
            confirmed,
            headings,
            statTiles,
            budgetRows: budgetRows.slice(0, 10),
            pills,
            budgetSub: budgetSub.slice(0, 6),
            periodText,
            addBtns,
            assignText,
            formLabels: formLabels.slice(0, 8),
            bars,
            subColor,
            overRows,
            aboveFold,
            totalBudgetRows: allBudgetDivs.length,
          };
        });

        fs.writeFileSync(
          path.join(SHOTS_DIR, "glamor_04_budgets_dom.json"),
          JSON.stringify(domInfo, null, 2)
        );
        console.log("  wrote glamor_04_budgets_dom.json");
        console.log("  DOM info:", JSON.stringify(domInfo, null, 2));
      }

      if (errors.length) {
        console.warn(`  page errors at ${width}/${theme}: ${errors.join(" | ")}`);
      }
      await ctx.close();
    }
  }
  console.log("\nGLAMOR G4: all screenshots captured.");
} finally {
  await browser.close();
}
