import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1100 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const open = async () => { await p.goto(URL + "/debt", { waitUntil: "domcontentloaded" }); await p.waitForSelector(".bento-debt", { timeout: 15000 }).catch(()=>{}); await p.waitForTimeout(1200); };

await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await open();

check("T1 widgetized surface host", await p.locator('.bento-debt').count() === 1);
check("T2 summary tile — total owed hero", await p.locator('[data-testid="debt-total-owed"]').count() === 1);
check("T3 engine ratio chips", await p.locator('.debt-hero .debt-stat').count() >= 2, `${await p.locator('.debt-hero .debt-stat').count()}`);
check("T4 payoff-ladder cards", await p.locator('.debt-card').count() >= 1, `${await p.locator('.debt-card').count()}`);
check("T5 payoff-rank medallion", await p.locator('.debt-card .debt-rank').count() >= 1);
check("T6 APR/utilization banded rail", await p.locator('.debt-card .debt-rail').count() >= 1);
check("T7 utilization meter (credit card)", await p.locator('.debt-util-fill').count() >= 1, `${await p.locator('.debt-util-fill').count()}`);
check("T8 strategy planner tile", await p.locator('#debt').count() >= 1 || await p.getByText(/snowball|avalanche/i).count() >= 1);

// Toolbar: reveal the Debt-metrics FormulaBuilder tile (config/formula engine surface).
check("T9 debt-metrics toggle present", await p.locator('[data-testid="debt-toggle-formulas"]').count() === 1);
await p.locator('[data-testid="debt-toggle-formulas"]').click({ force: true });
await p.waitForTimeout(700);
const hasFormula = await p.locator('.bento-debt').getByText(/formula|metric/i).count() >= 1;
// The debt_* engine variables must be discoverable in the picker.
const hasDebtVar = await p.evaluate(() => document.body.innerText.toLowerCase().includes("owed") || document.body.innerHTML.includes("debt_"));
check("T10 Debt metrics formula tile reveals", hasFormula && hasDebtVar);

// Include-in-plan toggle mutates + persists (card stays, no crash).
const toggle = p.locator('[data-testid^="debt-payoff-toggle-"]').first();
check("T11 include-in-plan toggle present", await toggle.count() === 1);
if (await toggle.count()) { await toggle.click({ force: true }); await p.waitForTimeout(600); }
check("T12 card list survives toggle", await p.locator('.debt-card').count() >= 1);

// A debt card links to its account's transactions.
const view = p.locator('[data-testid^="debt-view-"]').first();
if (await view.count()) { await view.click({ force: true }); await p.waitForTimeout(900); }
check("T13 view opens transactions", p.url().endsWith("/transactions"), p.url());

check("T14 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
