// /reports comprehensive e2e: the widgetized bento surface (hero, toolbar with view
// tabs / scope chips / metrics toggle / export menu, per-view section tiles) with
// positive AND negative cases — a nothing-matches scope keeps the toolbar on screen
// (no un-scope trap), Escape closes the export menu, the zeroed-categories disclosure
// folds the $0 rows, exports actually download, the reading posture (tab + YoY)
// survives a reload, and no page errors across the run.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1600 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const open = async () => { await p.goto(URL + "/reports", { waitUntil: "domcontentloaded" }); await p.waitForSelector(".bento-reports", { timeout: 15000 }).catch(() => {}); await p.waitForTimeout(1200); };
const seg = (label) => p.locator('.bento-reports [role="radio"], .bento-reports button', { hasText: label }).first();

// boot + sample
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await open();

// --- surface + hero ---
check("S1 widgetized surface host", await p.locator(".bento-reports").count() === 1);
const heroTxt = (await p.locator("#sec-hero .rpt-hero-value").first().innerText().catch(() => "")) || "";
check("S2 hero Net is a serif money figure", /[0-9]/.test(heroTxt), heroTxt.trim());
check("S3 hero carries at least 4 figure chips", await p.locator("#sec-hero .debt-stat").count() >= 4, `${await p.locator("#sec-hero .debt-stat").count()} chips`);
check("S4 toolbar: view tabs + scope + metrics + export", await p.locator('[data-testid="reports-scope-toggle"]').count() === 1 && await p.locator('[data-testid="reports-toggle-formulas"]').count() === 1 && await p.locator('[data-testid="reports-export-toggle"]').count() === 1);

// --- overview tiles ---
check("OV1 money-flow sankey renders (svg)", await p.locator("#sec-flow svg").count() >= 1);
check("OV2 payees + expenses tiles with ranked rows + drill links", await p.locator("#sec-payees .rows .row").count() >= 1 && await p.locator("#sec-expenses .rows .row").count() >= 1 && await p.locator('[data-testid="payees-drill"]').count() === 1);
check("OV3 income tile has the takeaway pull-quote", /income/i.test((await p.locator('[data-testid="income-takeaway"]').innerText().catch(() => "")) || ""));
check("OV4 ranked rows carry themed share bars", await p.locator(".bento-reports .share-bar .share-bar-fill").count() >= 2);

// --- scope filter (positive + the un-scope-trap negative) ---
await p.locator('[data-testid="reports-scope-toggle"]').click();
await p.waitForTimeout(400);
check("SC1 scope toggle reveals the chip filter", await p.locator(".scope-selector").count() === 1 && await p.locator(".scope-chip").count() >= 5, `${await p.locator(".scope-chip").count()} chips`);
// (neg) scope to a type that matches nothing → the toolbar must stay reachable.
const cryptoChip = p.locator(".scope-chip", { hasText: "Crypto" }).first();
if (await cryptoChip.count()) {
  await cryptoChip.click();
  await p.waitForTimeout(700);
  check("SC2 (neg) a nothing-matches scope keeps the toolbar on screen (no trap)", await p.locator('[data-testid="reports-scope-toggle"]').count() === 1 && await p.locator(".scope-chip-on").count() >= 1);
  await p.locator(".scope-chip-on").first().click(); // un-scope
  await p.waitForTimeout(700);
  check("SC3 clearing the scope restores the report", /[0-9]/.test((await p.locator("#sec-hero .rpt-hero-value").first().innerText().catch(() => "")) || ""));
} else {
  check("SC2 (neg) a nothing-matches scope keeps the toolbar on screen (no trap)", false, "no Crypto type chip found");
  check("SC3 clearing the scope restores the report", false, "skipped");
}
await p.locator('[data-testid="reports-scope-toggle"]').click();
await p.waitForTimeout(300);

// --- export menu (download + Escape-closes negative) ---
await p.locator('[data-testid="reports-export-toggle"]').click();
await p.waitForTimeout(300);
check("EX1 export menu opens with the CSV + PDF options", await p.locator('[data-testid="reports-export-category"]').isVisible().catch(() => false) && await p.locator('[data-testid="reports-export-pdf"]').count() === 1);
const [dl] = await Promise.all([
  p.waitForEvent("download", { timeout: 8000 }).catch(() => null),
  p.locator('[data-testid="reports-export-category"]').click(),
]);
check("EX2 exporting by-category downloads a period-stamped CSV", !!dl && /spending-by-category.*\.csv/.test(dl.suggestedFilename()), dl ? dl.suggestedFilename() : "no download");
await p.locator('[data-testid="reports-export-toggle"]').click();
await p.waitForTimeout(200);
await p.keyboard.press("Escape");
await p.waitForTimeout(300);
check("EX3 (neg) Escape closes the export menu", !(await p.locator('[data-testid="reports-export-category"]').isVisible().catch(() => false)));

// --- categories view: narrative, YoY/rollup, zeroed disclosure, drill ---
await seg("Categories").click({ force: true });
await p.waitForTimeout(900);
check("CA1 categories tile with the serif narrative pull-quote", await p.locator("#sec-categories").count() === 1 && /spent/i.test((await p.locator("#sec-categories .rpt-takeaway").innerText().catch(() => "")) || ""));
check("CA2 ranked category rows with drill buttons", await p.locator('[data-testid="reports-cat-row"]').count() >= 1 && await p.locator('[data-testid="reports-cat-drill"]').count() >= 1);
const zeroed = p.locator('[data-testid="reports-zeroed"]');
if (await zeroed.count()) {
  const rowsBefore = await p.locator('[data-testid="reports-cat-row"]:visible').count();
  await zeroed.locator("summary").click();
  await p.waitForTimeout(300);
  check("CA3 the zeroed-categories disclosure expands the $0 rows", await p.locator('[data-testid="reports-cat-row"]:visible').count() > rowsBefore, `${rowsBefore} → ${await p.locator('[data-testid="reports-cat-row"]:visible').count()}`);
  await zeroed.locator("summary").click();
} else {
  check("CA3 the zeroed-categories disclosure expands the $0 rows", true, "no zeroed categories in this dataset — n/a");
}
// YoY toggle flips its pressed state.
const yoy = p.locator('[data-testid="reports-yoy-toggle"]');
await yoy.click();
await p.waitForTimeout(400);
check("CA4 YoY toggle engages (aria-pressed)", (await yoy.getAttribute("aria-pressed")) === "true");
// Persistence: the tab + YoY survive a reload (kv → dataset autosave ticker ≈4s).
await p.waitForTimeout(4600);
await p.reload({ waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-reports", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1200);
check("CA5 the reading posture persists across a reload (Categories tab + YoY on)", await p.locator("#sec-categories").count() === 1 && (await p.locator('[data-testid="reports-yoy-toggle"]').getAttribute("aria-pressed")) === "true");
await p.locator('[data-testid="reports-yoy-toggle"]').click();
await p.waitForTimeout(300);
// Drill: category row → /transactions.
await p.locator('[data-testid="reports-cat-drill"]').first().click();
await p.waitForTimeout(900);
check("CA6 category drill lands on /transactions", p.url().includes("/transactions"));
await open();

// --- net worth view ---
await seg("Net worth").click({ force: true });
await p.waitForTimeout(900);
check("NW1 net-worth panel + paired trend tiles", await p.locator("#networth").count() >= 1 && await p.locator("#sec-cashtrend svg").count() >= 1 && await p.locator("#sec-savingstrend svg").count() >= 1);
check("NW2 trend takeaways read as sentences", /cash flow|savings rate/i.test(((await p.locator('[data-testid="cashflow-takeaway"]').innerText().catch(() => "")) || "") + ((await p.locator('[data-testid="savings-takeaway"]').innerText().catch(() => "")) || "")));

// --- advanced view ---
await seg("Advanced").click({ force: true });
await p.waitForTimeout(700);
const advOK = await p.locator('[data-testid="customfield-spend-section"]').count() === 1 || await p.locator('[data-testid="reports-tab-empty"]').count() === 1;
check("AD1 advanced view: custom-field section or a calm empty note", advOK);
if (await p.locator('[data-testid="cf-download-csv"]').count()) {
  const [dl2] = await Promise.all([
    p.waitForEvent("download", { timeout: 8000 }).catch(() => null),
    p.locator('[data-testid="cf-download-csv"]').click(),
  ]);
  check("AD2 custom-field CSV downloads", !!dl2, dl2 ? dl2.suggestedFilename() : "no download");
} else {
  check("AD2 custom-field CSV downloads", true, "no custom-field data — n/a");
}

// --- report metrics (custom values) ---
await seg("Overview").click({ force: true });
await p.waitForTimeout(600);
await p.locator('[data-testid="reports-toggle-formulas"]').click();
await p.waitForTimeout(900);
check("FM1 metrics toggle reveals the FormulaBuilder tile", await p.locator('[data-widget="rpt-formula"]').count() === 1);
check("FM2 the picker exposes the Reports variable group (report_* metrics)", /REPORTS/.test(await p.locator('[data-widget="rpt-formula"]').innerText().catch(() => "")) && /Monthly burn|Previous-period income/.test(await p.locator('[data-widget="rpt-formula"]').innerText().catch(() => "")));

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
