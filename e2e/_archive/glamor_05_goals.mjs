// GLAMOR G5 — Goals visual review ("Are We There Yet?" / Aaliyah).
// Persona: Aaliyah checks her goals to see at a glance: how close am I to each goal,
// am I on pace, and what should I fund next?
// Captures screenshots at 1280/1440/768 × dark + light themes.
// Saves to e2e/screenshots/ with names glamor_05_goals_<width>_<theme>.png.
// Light-theme recipe: confirmed working in G4 (post-boot patch of cashflux:prefs).
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

// bootWithTheme: boots WASM, injects theme via cashflux:prefs (solved recipe from G4),
// reloads, polls for data-theme, then navigates to /goals.
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

  // SOLVED LIGHT-THEME RECIPE (G4-confirmed): patch cashflux:prefs with the chosen theme and reload.
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

  // Reset "View as member" to Everyone if a member filter is present.
  try {
    const memberSel = await page.$('select[aria-label*="member"], select[aria-label*="Member"], .member-filter select');
    if (memberSel) {
      await memberSel.selectOption({ index: 0 });
      await page.waitForTimeout(300);
      console.log("  [ok] reset member filter to Everyone.");
    }
  } catch (_) {}

  // Navigate to /goals via the nav link.
  try {
    await page.locator('nav a[title="Goals"]').first().click();
    await page.waitForTimeout(800);
  } catch (_) {
    try {
      await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
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
        `glamor_05_goals_${width}_${theme}.png`
      );
      await page.screenshot({ path: shotPath, fullPage: false });
      console.log(`  wrote ${path.basename(shotPath)}`);

      // Full-page shot at 1280 dark.
      if (width === 1280 && theme === "dark") {
        const fullPath = path.join(
          SHOTS_DIR,
          `glamor_05_goals_${width}_${theme}_full.png`
        );
        await page.screenshot({ path: fullPath, fullPage: true });
        console.log(`  wrote ${path.basename(fullPath)}`);

        // DOM audit: harvest goal card structure, progress, pace, contribute affordance.
        const domInfo = await page.evaluate(() => {
          const confirmed = document.documentElement.getAttribute("data-theme");

          // Headings and section structure.
          const headings = Array.from(
            document.querySelectorAll("h1, h2, h3, .card-title, [class*='heading']")
          ).map((el) => el.innerText.trim()).filter(Boolean);

          // Goal rows/cards: .goal or rows with progress bars.
          const goalCards = Array.from(
            document.querySelectorAll(".goal, [class*='goal-row'], [class*='goal-card']")
          ).map((card) => {
            const nameEl = card.querySelector(".goal-name, .row-desc, h3, strong, [class*='name']");
            const progressBar = card.querySelector('[role="progressbar"], .bar, .progress, [class*="progress"]');
            const fill = card.querySelector('[class*="bar-fill"], [class*="fill"], [style*="width"]');
            const subLines = Array.from(card.querySelectorAll(".goal-sub, .sub, [class*='sub']")).map(
              (el) => el.innerText.trim()
            );
            const buttons = Array.from(card.querySelectorAll("button")).map(
              (b) => b.innerText.trim() || b.getAttribute("aria-label") || b.title
            );
            const paceEl = card.querySelector("[class*='pace'], [class*='on-track'], [class*='behind']");
            const rect = card.getBoundingClientRect();

            // Check for linked account chip.
            const linkedAccount = card.querySelector("[class*='account'], [class*='chip'], [class*='linked']");

            return {
              name: nameEl ? nameEl.innerText.trim().slice(0, 60) : "",
              hasProgressBar: !!progressBar,
              hasProgressFill: !!fill,
              fillStyle: fill ? (fill.getAttribute("style") || fill.className) : "",
              fillClass: fill ? fill.className : "",
              paceText: paceEl ? paceEl.innerText.trim() : "(no pace element)",
              subLines: subLines.slice(0, 4),
              buttons,
              linkedAccountText: linkedAccount ? linkedAccount.innerText.trim().slice(0, 40) : "(none)",
              cardHeight: Math.round(rect.height),
            };
          });

          // Fallback: look for any progress bars on page.
          const allBars = Array.from(
            document.querySelectorAll('[role="progressbar"]')
          ).map((b) => ({
            valuenow: b.getAttribute("aria-valuenow"),
            label: b.getAttribute("aria-label"),
            fillClass: b.firstElementChild ? b.firstElementChild.className : "",
          }));

          // Status summary pills.
          const pills = Array.from(
            document.querySelectorAll(".pill")
          ).map((p) => p.innerText.trim());

          // Summary stat tiles.
          const statTiles = Array.from(
            document.querySelectorAll(".stat-grid .stat, .stat-grid > *")
          ).map((el) => el.innerText.trim()).filter(Boolean);

          // Contribute / Fund buttons.
          const contributeButtons = Array.from(document.querySelectorAll("button"))
            .filter((b) => {
              const t = (b.innerText + (b.getAttribute("aria-label") || "") + (b.title || "")).toLowerCase();
              return t.includes("contribut") || t.includes("fund") || t.includes("add") || t.includes("deposit");
            })
            .map((b) => b.innerText.trim() || b.getAttribute("aria-label"));

          // Add-goal button.
          const addGoalBtns = Array.from(document.querySelectorAll("button"))
            .filter((b) => {
              const t = (b.innerText + (b.getAttribute("aria-label") || "") + (b.title || "")).toLowerCase();
              return t.includes("goal") || (t.includes("add") && t.includes("new"));
            })
            .map((b) => b.innerText.trim() || b.getAttribute("aria-label"));

          // Ordering: any ordering controls?
          const sortControls = Array.from(document.querySelectorAll("select, [class*='sort'], [class*='order']"))
            .map((el) => el.innerText.trim() || el.getAttribute("aria-label") || "").filter(Boolean);

          // Compute colors of goal names and progress figures for contrast check.
          const sampleGoal = document.querySelector(".goal, [class*='goal-row']");
          const sampleName = sampleGoal ? sampleGoal.querySelector(".goal-name, .row-desc, strong, [class*='name']") : null;
          const nameColor = sampleName ? getComputedStyle(sampleName).color : "(none)";

          const sampleSub = sampleGoal ? sampleGoal.querySelector(".goal-sub, .sub, [class*='sub']") : null;
          const subColor = sampleSub ? getComputedStyle(sampleSub).color : "(none)";

          // Number of goal cards above the fold.
          const allGoalDivs = Array.from(document.querySelectorAll(".goal, [class*='goal-row'], [class*='goal-card']"));
          const aboveFold = allGoalDivs.filter((r) => {
            const rect = r.getBoundingClientRect();
            return rect.top < window.innerHeight && rect.bottom > 0;
          }).length;

          // Check for empty state.
          const emptyEl = document.querySelector("[class*='empty'], [class*='cta'], .empty-state");
          const emptyText = emptyEl ? emptyEl.innerText.trim().slice(0, 80) : "(no empty state)";

          // Check for completion / celebration state on any goal.
          const completedGoals = allGoalDivs.filter((g) => {
            const t = g.innerText.toLowerCase();
            return t.includes("complete") || t.includes("achieved") || t.includes("100%") || t.includes("🎉") || t.includes("celebrate");
          });

          // Page-level structure: what's in the main content area.
          const mainEl = document.querySelector("main#main, main, [role='main']");
          const mainChildren = mainEl ? Array.from(mainEl.children).map(
            (c) => c.tagName + (c.className ? "." + c.className.split(" ").slice(0, 3).join(".") : "")
          ) : [];

          return {
            confirmed,
            headings,
            statTiles,
            goalCards: goalCards.slice(0, 10),
            allBars: allBars.slice(0, 10),
            pills,
            contributeButtons: contributeButtons.slice(0, 8),
            addGoalBtns: addGoalBtns.slice(0, 4),
            sortControls: sortControls.slice(0, 4),
            nameColor,
            subColor,
            aboveFold,
            totalGoalCards: allGoalDivs.length,
            emptyText,
            completedGoals: completedGoals.length,
            mainChildren: mainChildren.slice(0, 10),
          };
        });

        fs.writeFileSync(
          path.join(SHOTS_DIR, "glamor_05_goals_dom.json"),
          JSON.stringify(domInfo, null, 2)
        );
        console.log("  wrote glamor_05_goals_dom.json");
        console.log("  DOM info:", JSON.stringify(domInfo, null, 2));
      }

      if (errors.length) {
        console.warn(`  page errors at ${width}/${theme}: ${errors.join(" | ")}`);
      }
      await ctx.close();
    }
  }
  console.log("\nGLAMOR G5: all screenshots captured.");
} finally {
  await browser.close();
}
