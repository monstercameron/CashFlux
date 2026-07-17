// dash_period_contract_verify.mjs — locks the dashboard period contract
// (parity-scan defect: selecting another month left many widgets silently on
// "this month" with no label):
//   1. Current month: no "Today" badges; live figures.
//   2. Paged to a past month: recap recaps THAT month, cash flow ends on it,
//      current-state tiles wear the Today badge, hero says "as of today".
//   3. Paged to a future month: period-bound figures zero; recap shows the
//      empty state; badges present.
// Usage: node e2e/dash_period_contract_verify.mjs   (server on :8097)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 1400 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));

await page.goto(BASE + "/dashboard", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2500); // let deferred tiles mount

const badges = async () => await page.locator('[data-testid="w-today-badge"]').count();
const text = async (sel) => (await page.locator(sel).count()) ? (await page.locator(sel).first().innerText()).replace(/\s+/g, " ") : "";

// 1. Current month — no badges.
check("current month shows no Today badges", (await badges()) === 0, `${await badges()} badges`);

// 2. Page back one month (past).
await page.locator('[data-testid="period-prev"], button[aria-label*="Prev"], button[title*="Prev"]').first().click();
await page.waitForTimeout(2000);
const nBadges = await badges();
check("past month raises Today badges on current-state tiles", nBadges >= 8, `${nBadges} badges`);
const recapText = await text('[data-testid="monthly-recap"]');
check("recap recaps the SELECTED month (complete, no day range)", /June(?! \d)/.test(recapText) && !/July/.test(recapText), recapText.slice(0, 60));
const cfCaption = await text('[data-testid="cashflow-caption"]');
check("cash flow caption anchors on the selected month", /Jun/.test(cfCaption) && !/Jul/.test(cfCaption), cfCaption);
const hero = await text(".home-hero, [data-widget='hero']");
check("hero net worth is labeled as-of-today", /as of today/i.test(hero), hero.slice(0, 90));
await page.screenshot({ path: "e2e/dash_period_past.png", fullPage: true });

// 3. Forward two (future month).
const next = page.locator('[data-testid="period-next"], button[aria-label*="Next"], button[title*="Next"]').first();
await next.click(); await page.waitForTimeout(700);
await next.click(); await page.waitForTimeout(2000);
check("future month keeps the Today badges", (await badges()) >= 8, `${await badges()} badges`);
const heroF = await text(".home-hero, [data-widget='hero']");
check("future month zeroes period spending in the hero", /SPENDING \$0\.00/i.test(heroF.toUpperCase()), heroF.slice(0, 90));
await page.screenshot({ path: "e2e/dash_period_future.png", fullPage: false });

// Back to current.
await page.locator('[data-testid="period-prev"], button[aria-label*="Prev"], button[title*="Prev"]').first().click();
await page.waitForTimeout(1500);
check("back on the current month the badges clear", (await badges()) === 0, `${await badges()} badges`);

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
