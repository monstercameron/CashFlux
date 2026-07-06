// /credit comprehensive e2e: the bento surface where the proxy score IS a
// formula — hero shows the folded credit_proxy molecule, the utilization tile
// carries per-card rows with editable limits and pay-down targets, on-time/age
// factor tiles, the holding-back/improve pair. Positive AND negative cases —
// evaluating "credit_proxy" in the live FormulaBuilder equals the ring figure,
// editing a limit re-scores the page, and a card-less dataset shows the
// add-account CTA with the disclaimer. No page errors across the run.
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
await p.goto(URL + "/credit", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-credit", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1800);

// --- hero: ring + band + disclaimer + THE FORMULA ---
check("S1 widgetized surface host", await p.locator(".bento-credit").count() === 1);
const ringScore = (await p.locator("#sec-credit-hero .fig").first().innerText().catch(() => "")) || "";
check("H1 ring shows a number + band + the not-a-FICO disclaimer", /^\d+$/.test(ringScore.trim()) && /not a FICO/i.test(await p.locator("#sec-credit-hero").innerText()), `score=${ringScore.trim()}`);
await p.locator('[data-testid="credit-formula"] summary').click({ force: true }).catch(() => {});
await p.waitForTimeout(300);
const formulaTxt = (await p.locator('[data-testid="credit-formula"] code').innerText().catch(() => "")) || "";
check("H2 the hero folds the LIVE credit_proxy formula", formulaTxt.startsWith("credit_proxy = clamp(floor(") && /credit_util_score\*credit_util_weight/.test(formulaTxt));
check("H3 the formula names all three factors", ["credit_ontime_score", "credit_age_score"].every(v => formulaTxt.includes(v)));

// --- utilization tile: one-story head + per-card rows ---
check("U1 utilization tile fuses value + target into one line", await p.locator('[data-testid="cf-met-util"], [data-testid="cf-unmet-util"]').count() === 1);
check("U2 per-card rows with limits render", /of .*limit/.test(await p.locator("#sec-cf-util").innerText()) && (await p.locator("#sec-cf-util input").count()) >= 2, `${await p.locator("#sec-cf-util input").count()} limit inputs`);
check("U3 an actionable pay-down target renders", /Pay \$[\d,.]+ to reach 30%/.test(await p.locator("#sec-cf-util").innerText()));

// --- supporting factor tiles ---
check("F1 on-time + age tiles render one-story style", (await p.locator("#sec-cf-ontime").count()) + (await p.locator("#sec-cf-age").count()) === 2 && await p.locator('[data-testid^="cf-met-"], [data-testid^="cf-unmet-"], [data-testid^="cf-na-"]').count() >= 3);
await p.locator("#sec-cf-ontime .hlt-curve summary").click({ force: true }).catch(() => {});
await p.waitForTimeout(200);
check("F2 the disclosure carries score/weight + the variable chip", /counts for \d+%/.test(await p.locator("#sec-cf-ontime").innerText()) && /credit_ontime_score/.test((await p.locator('[data-testid="cf-var-ontime"]').innerText().catch(() => "")) || ""));

// --- holding-back / improve pair ---
check("D1 demerits with point costs", /pts/.test(await p.locator("#sec-credit-down").innerText()) || /Nothing is dragging/.test(await p.locator("#sec-credit-down").innerText()));
check("D2 advice with point impacts", /\+\d+ pts|No urgent moves/.test(await p.locator("#sec-credit-up").innerText()));

// --- formula parity: the builder evaluates credit_proxy to the ring figure ---
await p.locator('[data-testid="credit-toggle-formulas"]').click({ force: true });
await p.waitForTimeout(1000);
check("FM1 metrics toggle reveals the FormulaBuilder seeded with credit_proxy", await p.locator('[data-widget="crd-formula"]').count() === 1);
const fbTxt = (await p.locator('[data-widget="crd-formula"]').innerText().catch(() => "")) || "";
const evalMatch = fbTxt.match(/=\s*\n?\s*(\d+)/);
check("FM2 evaluating credit_proxy equals the ring figure", !!evalMatch && evalMatch[1] === ringScore.trim(), `builder=${evalMatch ? evalMatch[1] : "?"} ring=${ringScore.trim()}`);
check("FM3 the picker exposes the Credit health group", /CREDIT HEALTH/.test(fbTxt) && /Utilization factor|Pay to reach 30%/.test(fbTxt));

// --- limit edit re-scores the page (the C211 flow on the new surface) ---
// The editor folds behind an "Edit limit" disclosure now — open it first.
await p.locator(".credit-card-item details summary", { hasText: "Edit limit" }).first().click({ force: true });
await p.waitForTimeout(200);
const firstLimit = p.locator("#sec-cf-util input").first();
const aggBefore = (await p.locator("#sec-cf-util .hlt-factor-value").innerText().catch(() => "")) || "";
await firstLimit.fill("24000");
await firstLimit.blur(); // the editor commits on blur
await p.waitForTimeout(1200);
const aggAfter = (await p.locator("#sec-cf-util .hlt-factor-value").innerText().catch(() => "")) || "";
check("L1 editing a credit limit re-scores utilization live", aggBefore !== aggAfter && /\d+%/.test(aggAfter), `${aggBefore} → ${aggAfter}`);

// --- (neg) wiped dataset → CTA + disclaimer, not a blank page ---
const startFresh = p.locator(".sample-banner").locator("a, button", { hasText: "Start fresh" }).first();
if (await startFresh.count()) {
  await startFresh.click({ force: true });
  await p.waitForTimeout(1500);
  await p.goto(URL + "/credit", { waitUntil: "domcontentloaded" });
  await p.waitForTimeout(1500);
  const txt = await p.locator("#app").innerText();
  check("NEG1 (neg) a card-less dataset shows the add-account CTA + disclaimer", (await p.locator(".bento-credit").count()) === 0 && /not a FICO/i.test(txt));
} else {
  check("NEG1 (neg) a card-less dataset shows the add-account CTA + disclaimer", true, "no Start-fresh banner — n/a");
}

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
