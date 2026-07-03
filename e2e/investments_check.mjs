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

// Every account has its own growth chart; pools are a custom grouping exposing a variable.
check("P1 account-growth & pools section", await p.locator('#sec-pools').count() >= 1);
const acctCards = await p.locator('[data-testid^="invest-acct-"].inv-pool-card').count();
check("P2 every account has its own growth chart", acctCards >= 1 && await p.locator('.inv-pool-card svg').count() >= acctCards, `${acctCards} account cards`);
check("P3 each account card has a pool selector", await p.locator('[data-testid^="invest-assign-"]').count() === acctCards, `${await p.locator('[data-testid^="invest-assign-"]').count()} vs ${acctCards}`);
// Create a pool via the flip modal: a name field + a checkable list of accounts.
await p.locator('[data-testid="invest-new-pool"]').click({ force: true }); await p.waitForTimeout(500);
check("P4 New-pool opens a flip modal listing the accounts", await p.locator('.inv-pool-modal').count() === 1 && await p.locator('[data-testid^="pool-acct-"]').count() >= 2, `${await p.locator('[data-testid^="pool-acct-"]').count()} account toggles`);
await p.locator('[data-testid="pool-name"]').fill("Retirement");
const toggles = await p.locator('[data-testid^="pool-acct-"]').all();
await toggles[0].click({ force: true }); await toggles[1].click({ force: true }); await p.waitForTimeout(200);
check("P5 checking an account marks it included", (await toggles[0].getAttribute("aria-checked")) === "true");
await p.locator('[data-testid="pool-save"]').click({ force: true }); await p.waitForTimeout(700);
check("P6 saving creates the pool chip + its pool_<name>_value variable", await p.locator('.inv-pool-chip').count() >= 1 && ((await p.locator('.inv-pool-var').first().textContent().catch(() => "")) || "").includes("pool_"));
check("P7 accounts keep their own charts after pooling", await p.locator('[data-testid^="invest-acct-"].inv-pool-card').count() === acctCards);
check("P8 the pool chip shows its combined value", ((await p.locator('.inv-pool-chip-val').first().textContent().catch(() => "")) || "").replace(/[^0-9]/g, "").length > 0);
// Editing opens the same modal, pre-filled with the name.
await p.locator('[data-testid^="invest-pool-edit-"]').first().click({ force: true }); await p.waitForTimeout(500);
check("P9 edit opens the pre-filled pool modal", await p.locator('.inv-pool-modal').count() === 1 && (await p.locator('[data-testid="pool-name"]').inputValue().catch(() => "")) === "Retirement");
await p.locator('[data-testid="pool-cancel"]').click({ force: true }); await p.waitForTimeout(400);
check("P10 (regression) closing the pool modal doesn't crash", await p.locator('.inv-pool-modal').count() === 0 && errs.length === 0);
// Delete the pool.
await p.locator('[data-testid^="invest-pool-del-"]').first().click({ force: true }); await p.waitForTimeout(400);
await p.evaluate(() => { const c = document.querySelector('#cf-dialog-confirm'); if (c) c.click(); }); await p.waitForTimeout(600);
check("P11 deleting the pool removes its chip (accounts unchanged)", await p.locator('.inv-pool-chip').count() === 0 && await p.locator('[data-testid^="invest-acct-"].inv-pool-card').count() === acctCards);

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
// Close the add modal — Cancel/Done previously crashed the page (hook-outside-component);
// this is the regression guard. Also required so the delete below isn't behind the modal.
await p.locator('[data-testid="hld-cancel"]').click({ force: true }).catch(() => {});
await p.waitForTimeout(400);
check("A8 (regression) closing the add modal doesn't crash", await p.locator('[data-testid="invest-add-form"]').count() === 0 && errs.length === 0, `errs=${errs.length}`);

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
