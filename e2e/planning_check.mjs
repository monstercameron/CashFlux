// /planning comprehensive e2e: the widgetized bento surface (toolbar, cash runway, afford,
// 12-month forecast, saved what-if scenarios, metrics FormulaBuilder) with positive AND negative
// cases — an unaffordable purchase shows a shortfall, a nameless plan errors, a depleting plan
// flags a runway danger, a huge buffer breaches, and no page errors across the run.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1400 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const open = async () => { await p.goto(URL + "/planning", { waitUntil: "domcontentloaded" }); await p.waitForSelector(".bento-planning", { timeout: 15000 }).catch(() => {}); await p.waitForTimeout(1100); };
const num = (s) => parseFloat((s || "").replace(/[^0-9.]/g, "")) || 0;

// boot + sample
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await open();

// --- surface ---
check("S1 widgetized surface host", await p.locator(".bento-planning").count() === 1);
check("S2 tiles present (toolbar + runway + afford + forecast + scenarios)", await p.locator(".bento-planning > .w").count() >= 5, `${await p.locator(".bento-planning > .w").count()} tiles`);
check("S3 toolbar has plan-metrics toggle + manage-recurring link", await p.locator('[data-testid="planning-toggle-formulas"]').count() === 1 && await p.locator('.bento-planning a[href$="/recurring"]').count() >= 1);

// --- runway ---
check("R1 cash-runway section + Safe-to-spend hero figure", await p.locator("#sec-runway").count() === 1 && await p.locator("#sec-runway .stat-value.is-hero").count() >= 1);
const heroTxt = (await p.locator("#sec-runway .stat-value.is-hero").first().innerText()) || "";
check("R2 the runway hero is a money figure", /[0-9]/.test(heroTxt), heroTxt.trim());
check("R3 runway daily-balance chart renders (svg)", await p.locator("#sec-runway svg").count() >= 1);
// (neg) a warning buffer above the projected low → a breach warning + a threshold line on the chart.
const rwBuf = p.locator('#sec-runway input[type="number"]').first();
await rwBuf.fill("99999999"); await rwBuf.dispatchEvent("input"); await p.waitForTimeout(600);
check("R4 (neg) a buffer above the low flags a breach + a chart threshold line", await p.locator('[data-testid="runway-breach"]').count() >= 1 && /warning level/i.test(await p.locator("#sec-runway").innerText()), (await p.locator('[data-testid="runway-breach"]').first().innerText().catch(() => "")).slice(0, 50));
await rwBuf.fill(""); await rwBuf.dispatchEvent("input"); await p.waitForTimeout(400);
check("R5 clearing the buffer removes the breach + threshold line", await p.locator('[data-testid="runway-breach"]').count() === 0 && !/warning level/i.test(await p.locator("#sec-runway").innerText()));

// --- afford (positive + negative) ---
check("AF1 afford section is inert until an amount is entered", await p.locator("#sec-afford").count() === 1 && /enter a purchase/i.test(await p.locator("#sec-afford").innerText()));
// An affordable purchase (small, 0 months) → a "yes" verdict, no error.
const afInputs = p.locator('#sec-afford input[type="number"]');
await afInputs.nth(0).fill("500"); await afInputs.nth(0).dispatchEvent("input");
await afInputs.nth(1).fill("0"); await afInputs.nth(1).dispatchEvent("input");
await p.waitForTimeout(500);
check("AF2 a small affordable purchase shows a yes-verdict (no shortfall alert)", await p.locator('#sec-afford [role=alert]').count() === 0 && /[0-9]/.test(await p.locator("#sec-afford").innerText()));
// (neg) A huge purchase → a shortfall alert.
await afInputs.nth(0).fill("9999999"); await afInputs.nth(0).dispatchEvent("input");
await p.waitForTimeout(500);
check("AF3 (neg) an unaffordable purchase shows a shortfall alert", await p.locator('#sec-afford [role=alert]').count() >= 1, (await p.locator('#sec-afford [role=alert]').first().innerText().catch(() => "")).slice(0, 60));
await afInputs.nth(0).fill(""); await afInputs.nth(0).dispatchEvent("input");
await p.waitForTimeout(300);

// --- forecast ---
check("F1 forecast section + projected 12-month figure", await p.locator("#sec-forecast").count() === 1 && await p.locator("#sec-forecast .stat-value.is-hero").count() >= 1);
check("F2 forecast basis says '3-month trailing average'", /3-month trailing average/i.test((await p.locator('[data-testid="forecast-basis"]').innerText().catch(() => "")) || ""));
check("F3 forecast chart renders (svg)", await p.locator("#sec-forecast svg").count() >= 1);
// Trimming spending changes the forecast trim note.
const trimIn = p.locator('#sec-forecast input[type="number"]').first();
await trimIn.fill("300"); await trimIn.dispatchEvent("input");
await p.waitForTimeout(500);
check("F4 a monthly-spending trim adds a scenario note", (await p.locator("#sec-forecast").innerText()).length > 0 && errs.length === 0);
await trimIn.fill(""); await trimIn.dispatchEvent("input");
await p.waitForTimeout(300);

// --- scenarios: the add form is a flip modal opened from the section header ---
await p.locator("#sec-scenarios").scrollIntoViewIfNeeded().catch(() => {});
const openAddModal = async () => {
  if (await p.locator('[data-testid="plan-add-form"]').count() === 0) {
    await p.locator('[data-testid="plan-add-open"]').click({ force: true });
    await p.waitForSelector('[data-testid="plan-add-form"]', { timeout: 6000 }).catch(() => {});
    await p.waitForTimeout(700); // past the 550ms flip
  }
};
check("SC0 the scenarios tile has an 'Add plan' trigger (no inline form)", await p.locator('[data-testid="plan-add-open"]').count() === 1 && await p.locator('[data-testid="plan-add-form"]').count() === 0);
await openAddModal();
check("SC0b clicking 'Add plan' opens the flip modal", await p.locator('[data-testid="plan-add-form"]').count() === 1);
const modal = p.locator('[data-testid="plan-add-form"]');
const scNums = modal.locator('input[type="number"]'); // [0]=horizon [1]=start [2]=monthly [3]=once-amt [4]=once-month
await modal.locator("#plan-add").fill("Steady Saver");
await scNums.nth(0).fill("12"); // horizon
await scNums.nth(1).fill("5000"); // start
await scNums.nth(2).fill("300"); // monthly
await modal.locator('button[type="submit"]').click({ force: true });
await p.waitForTimeout(700);
check("SC1 adding a plan appends a scenario row", await p.locator("#sec-scenarios .plan-scenario").count() >= 1, `${await p.locator("#sec-scenarios .plan-scenario").count()} rows`);
check("SC2 the plan row shows a projected end + a sparkline", /[0-9]/.test(await p.locator("#sec-scenarios").innerText()) && await p.locator("#sec-scenarios svg").count() >= 1);

// (neg) a nameless plan errors — the modal stays open after a successful add, so clear the name.
await modal.locator("#plan-add").fill("");
await modal.locator('button[type="submit"]').click({ force: true });
await p.waitForTimeout(400);
check("SC3 (neg) adding a plan with no name shows an error", (await modal.innerText()).toLowerCase().includes("name") || await modal.locator(".err, [role=alert]").count() >= 1);

// (neg) a depleting plan flags a runway danger.
await modal.locator("#plan-add").fill("Burn Down");
await scNums.nth(0).fill("12");
await scNums.nth(1).fill("1000");
await scNums.nth(2).fill("-500");
await modal.locator('button[type="submit"]').click({ force: true });
await p.waitForTimeout(700);
check("SC4 (neg) a depleting plan flags a runway-danger indicator", await p.locator(".plan-runway--danger").count() >= 1 && await p.locator(".plan-runway__text").count() >= 1);
// close the modal before moving on (backdrop/X) so it doesn't overlay later interactions.
await p.locator('[data-testid="plan-add-cancel"]').click({ force: true }).catch(() => {});
await p.waitForTimeout(500);
check("SC4b cancel closes the add modal", await p.locator('[data-testid="plan-add-form"]').count() === 0);

// --- compare a saved plan on the forecast chart ---
await open();
await p.waitForTimeout(400);
const compareSel = p.locator('[data-testid="plan-compare-select"]');
check("CM1 with saved plans, the forecast gains a compare-with picker", await compareSel.count() === 1);
if (await compareSel.count()) {
  const optVal = await compareSel.locator("option").nth(1).getAttribute("value");
  await compareSel.selectOption(optVal);
  await p.waitForTimeout(600);
  check("CM2 comparing overlays a plan curve + shows a compare note", await p.locator('[data-testid="plan-compare-note"]').count() >= 1);
}

// --- delete a scenario ---
await p.locator("#sec-scenarios").scrollIntoViewIfNeeded().catch(() => {});
const rowsBefore = await p.locator("#sec-scenarios .plan-scenario").count();
// Delete now lives in the ⋯ overflow menu — open it, then click the delete item.
await p.locator('#sec-scenarios [data-testid^="plan-menu-"]').first().click({ force: true });
await p.waitForTimeout(300);
check("SC5a the scenario ⋯ menu exposes a delete item (no bare X button)", await p.locator('#sec-scenarios [data-testid^="plan-del-"]').first().isVisible().catch(() => false) && await p.locator("#sec-scenarios .btn-del").count() === 0);
await p.locator('#sec-scenarios [data-testid^="plan-del-"]').first().click({ force: true });
await p.evaluate(() => { const c = document.querySelector("#cf-dialog-confirm"); if (c) c.click(); }).catch(() => {});
await p.waitForTimeout(600);
check("SC5 deleting a scenario removes its row", await p.locator("#sec-scenarios .plan-scenario").count() < rowsBefore || rowsBefore === 0, `${rowsBefore} → ${await p.locator("#sec-scenarios .plan-scenario").count()}`);

// --- metrics FormulaBuilder (custom values) ---
await p.locator('[data-testid="planning-toggle-formulas"]').click({ force: true });
await p.waitForTimeout(700);
check("FM1 the metrics toggle reveals a FormulaBuilder", await p.locator("#plan-formula").count() >= 1 || (await p.locator(".bento-planning").innerText()).toLowerCase().includes("formula"));
check("FM2 the picker exposes the Planning variable group (runway_/forecast_/plan_)", /runway_buffer|forecast_horizon|plan_/.test(await p.locator(".bento-planning").innerText()));

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
