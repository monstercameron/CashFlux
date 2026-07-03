// /health comprehensive e2e: the widgetized bento surface where the score IS a
// formula — the hero shows the live health_score molecule, each factor tile is
// in-depth (value vs target, meter, contribution, why, curve, live variable
// chip) and addressable (act drill), plus focus-next steps and the opt-in
// FormulaBuilder. Positive AND negative cases — the formula text matches the
// molecule, evaluating "health_score" in the builder equals the ring figure,
// an act drill navigates, and a wiped dataset shows a calm no-data state.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 2000 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));

// boot + sample
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto(URL + "/health", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-health", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1800);

// --- hero: ring + band + THE FORMULA ---
check("S1 widgetized surface host", await p.locator(".bento-health").count() === 1);
const ringScore = (await p.locator("#sec-health-hero .fig").first().innerText().catch(() => "")) || "";
check("H1 score ring shows a number + band", /^\d+$/.test(ringScore.trim()) && /(Excellent|Good|Fair|Needs work|Critical)/.test(await p.locator("#sec-health-hero").innerText()), `score=${ringScore.trim()}`);
// The formula folds behind a disclosure to keep the hero glanceable — open it.
await p.locator('[data-testid="health-formula"] summary').click({ force: true }).catch(() => {});
await p.waitForTimeout(300);
const formulaTxt = (await p.locator('[data-testid="health-formula"] code').innerText().catch(() => "")) || "";
check("H2 the hero shows the LIVE health_score formula", formulaTxt.startsWith("health_score = clamp(round(") && /health_savings\*health_savings_weight/.test(formulaTxt));
check("H3 the formula names all six factors + the penalty", ["health_emergency", "health_debt", "health_budget", "health_utilization", "health_trend", "health_penalty"].every(v => formulaTxt.includes(v)));

// --- factor tiles: in-depth + addressable ---
const factorKeys = ["savings", "emergency", "debt", "budget", "utilization", "nw-trend"];
let tileCount = 0;
for (const k of factorKeys) { if (await p.locator(`#sec-hf-${k}`).count()) tileCount++; }
check("F1 all six factor tiles render", tileCount === 6, `${tileCount}/6`);
check("F2 factor tiles carry meters + contribution shares", await p.locator(".bento-health .pb, .bento-health [role=progressbar], .bento-health .progress").count() >= 4 || /of your score/.test(await p.locator(".bento-health").innerText()));
check("F3 each applicable factor shows its live variable chip", await p.locator('[data-testid^="hf-var-"]').count() >= 4, `${await p.locator('[data-testid^="hf-var-"]').count()} chips`);
const chipTxt = (await p.locator('[data-testid="hf-var-savings"]').innerText().catch(() => "")) || "";
check("F4 the chip pairs the variable name with its live value", /health_savings/.test(chipTxt) && /\d/.test(chipTxt), chipTxt.trim());
// The exact scoring curve folds behind a per-tile "How it's scored" disclosure.
await p.locator("#sec-hf-savings .hlt-curve summary").click({ force: true }).catch(() => {});
await p.waitForTimeout(200);
check("F5 tiles explain WHY + the scoring curve behind its disclosure", /Scored 0/.test(await p.locator("#sec-hf-savings").innerText()) && /engine of everything else/.test(await p.locator("#sec-hf-savings").innerText()));

// The formula builder evaluates health_score to the SAME number the ring shows.
await p.locator('[data-testid="health-toggle-formulas"]').click({ force: true });
await p.waitForTimeout(1000);
check("FM1 metrics toggle reveals the FormulaBuilder seeded with health_score", await p.locator('[data-widget="hlt-formula-builder"]').count() === 1);
const fbTxt = (await p.locator('[data-widget="hlt-formula-builder"]').innerText().catch(() => "")) || "";
const evalMatch = fbTxt.match(/=\s*\n?\s*(\d+)/);
check("FM2 evaluating health_score in the builder equals the ring figure", !!evalMatch && evalMatch[1] === ringScore.trim(), `builder=${evalMatch ? evalMatch[1] : "?"} ring=${ringScore.trim()}`);
check("FM3 the picker exposes the Health factors group", /HEALTH FACTORS/.test(fbTxt) && /Savings factor|Deficit penalty/.test(fbTxt));

// --- act drill navigates (addressable) ---
await p.locator('[data-testid="hf-act-debt"]').click({ force: true });
await p.waitForTimeout(900);
check("A1 'Act on this' (debt) lands on /debt", p.url().includes("/debt"));
await p.goto(URL + "/health", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-health", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1200);

// --- focus-next steps still drill (kept testid) ---
if (await p.locator('[data-testid="health-step"]').count()) {
  await p.locator('[data-testid="health-step"]').first().click({ force: true });
  await p.waitForTimeout(900);
  check("A2 a focus-next step drills to its screen", !p.url().endsWith("/health"));
  await p.goto(URL + "/health", { waitUntil: "domcontentloaded" });
  await p.waitForTimeout(1000);
} else {
  check("A2 a focus-next step drills to its screen", true, "no steps (all factors ≥90) — n/a");
}

// --- (neg) wiped dataset → calm no-data state, page still renders ---
const startFresh = p.locator(".sample-banner").locator("a, button", { hasText: "Start fresh" }).first();
if (await startFresh.count()) {
  await startFresh.click({ force: true });
  await p.waitForTimeout(1500);
  await p.goto(URL + "/health", { waitUntil: "domcontentloaded" });
  await p.waitForTimeout(1500);
  const txt = await p.locator("#app").innerText();
  check("NEG1 (neg) an empty dataset reads 'Not enough data', not a fake score", /Not enough data/i.test(txt) && !/Excellent|Critical/.test((await p.locator("#sec-health-hero .fig").innerText().catch(() => "")) || ""));
} else {
  check("NEG1 (neg) an empty dataset reads 'Not enough data', not a fake score", true, "no Start-fresh banner — n/a");
}

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
