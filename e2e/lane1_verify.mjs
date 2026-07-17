// lane1_verify.mjs — end-to-end verification of the reports-trust lane fixes:
//   #47 / QA CF-01/UX-03 — a specific-account report scope actually narrows every
//        masthead figure, shows a plain-language "Showing X only" sentence that
//        stays visible with the Scope panel closed, and offers a one-click Reset.
// Usage: node e2e/lane1_verify.mjs   (server on :8111 serving the lane1 webroot)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8111";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));

const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1400); };

// Read the four masthead figures (label → value) from the reports hero.
const heroFigs = async () => {
  const figs = {};
  for (const f of await page.locator('[data-testid="rpt-hero"] .rpta-fig').all()) {
    const k = await f.locator(".rpta-fig-k").innerText();
    figs[k.trim()] = (await f.locator(".rpta-fig-v").innerText()).trim();
  }
  return figs;
};

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

// ───────────── #47: specific-account scope narrows the whole report ─────────────
await nav("/reports");
const all = await heroFigs();
check("#47 setup: masthead figures render household-wide", Object.keys(all).length >= 3, JSON.stringify(all));
check("#47 setup: no scope sentence while unscoped", (await page.locator('[data-testid="rpta-scope-line"]').count()) === 0);

// Open the Scope panel and tick exactly ONE account.
await page.locator('[data-testid="reports-scope-toggle"]').click();
await page.waitForTimeout(700);
await page.locator('[data-testid="scope-selector"] button:has-text("Specific accounts")').click();
await page.waitForTimeout(500);
const firstRow = page.locator(".scope-acct-row").first();
const acctName = (await firstRow.innerText()).trim();
await firstRow.locator('input[type="checkbox"]').check();
await page.waitForTimeout(1600);

const scoped = await heroFigs();
const changed = Object.keys(all).filter((k) => scoped[k] !== undefined && scoped[k] !== all[k]);
check("#47: masthead figures recalculate under a one-account scope", changed.length >= 2,
  `changed=[${changed.join(", ")}] all=${JSON.stringify(all)} scoped=${JSON.stringify(scoped)}`);

// The plain-language sentence + reset, visible even with the panel CLOSED.
await page.locator('[data-testid="reports-scope-toggle"]').click(); // close the panel
await page.waitForTimeout(700);
const line = page.locator('[data-testid="rpta-scope-line"]');
check("#47: scope sentence visible with the panel closed", (await line.count()) === 1);
if (await line.count()) {
  const txt = (await line.innerText()).replace(/\n/g, " ");
  check("#47: sentence names the scoped account in plain English",
    txt.includes("Showing") && txt.includes(acctName) && txt.includes("only"), txt);
  check("#47: sentence flags household-wide figures as unscoped", /household-wide/i.test(txt), txt);
}

// One-click reset restores the household view.
await page.locator('[data-testid="rpta-scope-reset"]').click();
await page.waitForTimeout(1600);
const resetFigs = await heroFigs();
const restored = Object.keys(all).every((k) => resetFigs[k] === all[k]);
check("#47: Reset scope restores every household figure", restored,
  `all=${JSON.stringify(all)} reset=${JSON.stringify(resetFigs)}`);
check("#47: scope sentence gone after reset", (await page.locator('[data-testid="rpta-scope-line"]').count()) === 0);

check("no page errors", errors.length === 0, errors.slice(0, 3).join(" | "));

await browser.close();
console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
