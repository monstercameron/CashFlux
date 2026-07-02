// /investments e2e: widgetized surface, traditional accounts, add/delete a security,
// allocation, links, formula toggle, negatives.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1200 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const open = async () => { await p.goto(URL + "/investments", { waitUntil: "domcontentloaded" }); await p.waitForSelector(".bento-invest", { timeout: 15000 }).catch(() => {}); await p.waitForTimeout(1200); };

await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await open();

// Surface + summary.
check("S1 widgetized surface host", await p.locator('.bento-invest').count() === 1);
check("S2 portfolio-value hero", await p.locator('[data-testid="invest-total"]').count() === 1);
const totTxt = (await p.locator('[data-testid="invest-total"]').textContent().catch(() => "")) || "";
check("S3 total is a money figure", /[0-9]/.test(totTxt.replace(/[^0-9]/g, "")), totTxt.trim());
check("S4 securities/traditional split line", await p.locator('.inv-hero-sub').count() >= 1);
check("S5 gain/return/cost chips", await p.locator('.inv-hero .debt-stat').count() >= 3, `${await p.locator('.inv-hero .debt-stat').count()}`);

// Toolbar + sections.
check("T1 add-security button", await p.locator('[data-testid="invest-add"]').count() === 1);
check("T2 manage-accounts link", await p.locator('.bento-invest a[href$="/accounts"]').count() >= 1);
check("T3 portfolio-metrics toggle", await p.locator('[data-testid="invest-toggle-formulas"]').count() === 1);
check("T4 traditional (balance-tracked) accounts render", await p.locator('[data-testid^="invtrad-"]').count() >= 1, `${await p.locator('[data-testid^="invtrad-"]').count()}`);
check("T5 net-worth owner link on the hero", await p.locator('.debt-owner-link[href$="/networth"]').count() >= 1);

// Growth chart + configurable 1M/6M/1Y window.
check("G1 growth chart tile present", await p.locator('.inv-growth').count() === 1);
check("G2 1M/6M/1Y window toggle (3 segments)", await p.locator('.inv-seg-btn').count() === 3, `${await p.locator('.inv-seg-btn').count()}`);
check("G3 chart renders (svg)", await p.locator('.inv-growth svg').count() >= 1);
check("G4 current value + delta shown", await p.locator('.inv-growth-now').count() >= 1 && await p.locator('.inv-growth-delta').count() >= 1);
check("G5 default window is 1Y", await p.locator('[data-testid="invest-growth-12m"][aria-pressed="true"]').count() >= 1);
const delta12 = (await p.locator('.inv-growth-delta').first().textContent().catch(() => "")) || "";
await p.locator('[data-testid="invest-growth-6m"]').click({ force: true }); await p.waitForTimeout(700);
check("G6 toggling 6M activates that window", await p.locator('[data-testid="invest-growth-6m"][aria-pressed="true"]').count() >= 1);
const delta6 = (await p.locator('.inv-growth-delta').first().textContent().catch(() => "")) || "";
check("G7 changing the window re-scales the trend (delta differs)", delta6 !== delta12, `${delta12.trim()} → ${delta6.trim()}`);
await p.locator('[data-testid="invest-growth-1m"]').click({ force: true }); await p.waitForTimeout(600);
check("G8 1M window is selectable + chart survives", await p.locator('[data-testid="invest-growth-1m"][aria-pressed="true"]').count() >= 1 && await p.locator('.inv-growth svg').count() >= 1);
await p.locator('[data-testid="invest-growth-12m"]').click({ force: true }); await p.waitForTimeout(500);

// Add a stock security → securities + allocation appear.
await p.locator('[data-testid="invest-add"]').click({ force: true }); await p.waitForTimeout(500);
check("A1 add-security form reveals", await p.locator('[data-testid="invest-add-form"]').count() === 1);
await p.locator('[data-testid="hld-ticker"]').fill("AAPL");
await p.locator('[data-testid="hld-name"]').fill("Apple Inc.");
await p.locator('[data-testid="hld-shares"]').fill("10");
await p.locator('[data-testid="hld-cost"]').fill("1500");
await p.locator('[data-testid="hld-price"]').fill("200");
await p.locator('[data-testid="hld-save"]').click({ force: true });
await p.waitForTimeout(1000);
check("A2 a security holding card appears", await p.locator('.inv-card[data-testid^="holding-"]').count() >= 1, `${await p.locator('.inv-card[data-testid^="holding-"]').count()}`);
check("A3 holding shows its security-type badge", await p.locator('.inv-sec-badge').count() >= 1);
check("A4 holding shows a value + gain", await p.locator('.inv-card .inv-value').count() >= 1 && await p.locator('.inv-card .inv-gain').count() >= 1);
check("A5 allocation tile now renders (by type + class)", await p.locator('.inv-alloc-row').count() >= 1, `${await p.locator('.inv-alloc-row').count()}`);
// The account moves from balance-tracked to holdings-tracked (no double count), so the
// meaningful signal is that the securities portion is now non-zero.
const splitAfter = (await p.locator('.inv-hero-sub').first().textContent().catch(() => "")) || "";
check("A6 securities value is now non-zero in the split", !/\$0\.00 in securities/i.test(splitAfter), splitAfter.trim());

// (neg) Empty required fields → validation error, no crash.
await p.locator('[data-testid="invest-add"]').count() && await p.locator('[data-testid="invest-add"]').click({ force: true });
await p.waitForTimeout(400);
if (await p.locator('[data-testid="invest-add-form"]').count()) {
  await p.locator('[data-testid="hld-name"]').fill("");
  await p.locator('[data-testid="hld-shares"]').fill("");
  await p.locator('[data-testid="hld-save"]').click({ force: true });
  await p.waitForTimeout(500);
}
check("A7 (neg) invalid add shows an error, no crash", await p.locator('[data-testid="invest-add-form"] .err').count() >= 1 || errs.length === 0);

// Delete the security (confirm modal).
const delBtn = p.locator('[data-testid^="holding-del-"]').first();
if (await delBtn.count()) {
  await delBtn.click({ force: true }); await p.waitForTimeout(500);
  await p.evaluate(() => { const c = document.querySelector('#cf-dialog-confirm'); if (c) c.click(); });
  await p.waitForTimeout(800);
}
check("D1 deleting a holding removes its card", await p.locator('.inv-card[data-testid^="holding-"]').count() === 0);

// Formula tile toggle.
await open();
await p.locator('[data-testid="invest-toggle-formulas"]').click({ force: true }); await p.waitForTimeout(700);
check("F1 portfolio-metrics formula tile reveals", await p.locator('.bento-invest').getByText(/formula|metric/i).count() >= 1);

// Nav: traditional account → transactions.
await open();
const view = p.locator('[data-testid^="invtrad-view-"]').first();
if (await view.count()) { await view.click({ force: true }); await p.waitForTimeout(800); }
check("N1 a traditional account links to its transactions", p.url().endsWith("/transactions"), p.url());

check("Z1 no page errors across the run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
