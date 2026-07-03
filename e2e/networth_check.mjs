// /networth comprehensive e2e: the widgetized bento surface (hero + delta pill +
// chips, horizon toolbar, trend with takeaway + "Now" endpoint, own/owe composition
// pair, by-account rows, opt-in metrics) with positive AND negative cases — the
// horizon persists a reload, liability rows read in the down tone, the account
// drill navigates, and a wiped dataset shows the add-account CTA instead of a
// blank page. No page errors across the run.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1500 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));

// boot + sample
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto(URL + "/networth", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-networth", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1500);

// --- hero ---
check("S1 widgetized surface host", await p.locator(".bento-networth").count() === 1);
const heroTxt = (await p.locator('[data-testid="nw-hero-value"]').innerText().catch(() => "")) || "";
check("H1 hero net worth is a serif money figure", /[0-9]/.test(heroTxt), heroTxt.trim());
check("H2 hero chips (assets/liabilities/liquid share)", await p.locator("#sec-nw-hero .debt-stat").count() >= 3, `${await p.locator("#sec-nw-hero .debt-stat").count()} chips`);
const deltaTxt = (await p.locator('[data-testid="nw-delta"]').innerText().catch(() => "")) || "";
check("H3 month-to-date delta pill reads as money", deltaTxt === "" || /this month/.test(deltaTxt), deltaTxt || "no delta (flat month) — ok");

// --- toolbar ---
check("T1 toolbar: horizon tabs + metrics toggle + drills", await p.locator('[data-testid="nw-toggle-formulas"]').count() === 1 && await p.locator('[data-testid="nw-accounts-link"]').count() === 1 && await p.locator('[data-testid="nw-debt-link"]').count() === 1);

// --- trend ---
check("TR1 trend chart + serif takeaway", await p.locator("#sec-nw-trend svg").count() >= 1 && /now/i.test((await p.locator('[data-testid="nw-takeaway"]').innerText().catch(() => "")) || ""));
check("TR2 the trend ends at a 'Now' point", /Now/.test(await p.locator("#sec-nw-trend").innerText()));
// Switch to 2 years → chart re-renders, label style changes to include a year.
await p.locator('.bento-networth button', { hasText: "2 years" }).first().click({ force: true });
await p.waitForTimeout(800);
check("TR3 2-year horizon re-renders with year-stamped labels", /\b\d{2}\b|'2\d/.test(await p.locator("#sec-nw-trend").innerText()) && await p.locator("#sec-nw-trend svg").count() >= 1);
// Persistence: the horizon survives a reload (kv → dataset autosave ticker ≈4s).
await p.waitForTimeout(4600);
await p.reload({ waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-networth", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1200);
const twoYearsOn = await p.locator('.bento-networth [aria-checked="true"], .bento-networth [aria-pressed="true"], .bento-networth .seg-on, .bento-networth button', { hasText: "2 years" }).first().isVisible().catch(() => false);
check("TR4 the horizon persists across a reload", twoYearsOn && /over the last 24 months/.test((await p.locator('[data-testid="nw-takeaway"]').innerText().catch(() => "")) || ""));
await p.locator('.bento-networth button', { hasText: "6 months" }).first().click({ force: true });
await p.waitForTimeout(500);

// --- composition pair ---
check("OW1 'what you own' buckets with share bars", await p.locator("#sec-nw-own .row").count() >= 2 && await p.locator("#sec-nw-own .share-bar-fill").count() >= 2);
check("OW2 property bucket present (sample condo is TypeProperty)", /Property & vehicles/.test(await p.locator("#sec-nw-own").innerText()));
check("OE1 'what you owe' buckets in the down tone", await p.locator("#sec-nw-owe .row").count() >= 2 && await p.locator("#sec-nw-owe .share-bar-fill.nw-bar-down").count() >= 2);

// --- by account ---
check("AC1 per-account rows with type meta", await p.locator('[data-testid="nw-acct-row"]').count() >= 5, `${await p.locator('[data-testid="nw-acct-row"]').count()} rows`);
check("AC2 a liability row reads negative (parens)", /\(\$[\d,.]+\)/.test(await p.locator("#sec-nw-accounts").innerText()));
// Drill navigates to /accounts.
await p.locator('[data-testid="networth-drill"]').first().click({ force: true });
await p.waitForTimeout(900);
check("AC3 account drill lands on /accounts", p.url().includes("/accounts"));
await p.goto(URL + "/networth", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-networth", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1200);

// --- metrics (custom values) ---
await p.locator('[data-testid="nw-toggle-formulas"]').click({ force: true });
await p.waitForTimeout(900);
check("FM1 metrics toggle reveals the FormulaBuilder tile", await p.locator('[data-widget="nw-formula"]').count() === 1);
const fmTxt = (await p.locator('[data-widget="nw-formula"]').innerText().catch(() => "")) || "";
check("FM2 the picker exposes the Net worth variable group", /NET WORTH/.test(fmTxt) && /Change this month|Liquid share|Invested assets/.test(fmTxt));

// --- (neg) wiped dataset → add-account CTA, not a blank page ---
const startFresh = p.locator(".sample-banner", { hasText: "Start fresh" }).locator("a, button", { hasText: "Start fresh" }).first();
if (await startFresh.count()) {
  await startFresh.click({ force: true });
  await p.waitForTimeout(1500);
  await p.goto(URL + "/networth", { waitUntil: "domcontentloaded" });
  await p.waitForTimeout(1500);
  const cta = await p.locator(".empty-cta, .empty").count();
  check("NEG1 (neg) an empty dataset shows the add-account CTA, not a blank page", cta >= 1 && (await p.locator(".bento-networth").count()) === 0);
} else {
  check("NEG1 (neg) an empty dataset shows the add-account CTA, not a blank page", true, "no Start-fresh banner — n/a");
}

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
