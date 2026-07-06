// /assistant Insights briefing e2e: the tab is a widgetized bento surface — a
// hero tile (MTD spend + pace pill + agent brief + figure chips), a toolbar
// (custom-values toggle + drills), the attention pair (flagged all-clear /
// category shifts), the accent trend with takeaway, the merchants + pinned
// pair, the opt-in FormulaBuilder with the assistant_* picker group, the
// merchant drill-through, and the no-data empty state. Exits non-zero on any
// failure.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1200 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));

// --- boot + sample data ---
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto(URL + "/assistant", { waitUntil: "domcontentloaded" });
await p.waitForSelector('[data-testid="assistant-hub"]', { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1000);
await p.locator(".seg-btn", { hasText: /^Insights$/ }).first().click(); await p.waitForTimeout(1000);

// --- the surface is a bento grid of tiles ---
const grid = await p.evaluate(() => {
  const s = document.querySelector('[data-testid="assistant-insights-surface"]');
  if (!s) return null;
  const cs = getComputedStyle(s);
  return { display: cs.display, tiles: s.querySelectorAll(":scope > .w").length };
});
check("S1 the Insights tab renders the briefing bento surface", !!grid && grid.display === "grid", JSON.stringify(grid));
check("S2 seven tiles by default (hero/toolbar/attention pair/trend/merchants/pins)", grid && grid.tiles === 7, `tiles=${grid?.tiles}`);

// --- hero tile ---
const heroVal = await p.locator('[data-testid="ast-hero-value"]').innerText().catch(() => "");
check("H1 the hero states the month-to-date spend as a money figure", /\$[\d,]+\.\d\d/.test(heroVal), heroVal);
check("H2 the agent brief line reads in plain English", /You've spent|No spending recorded/.test(await p.locator('[data-testid="ast-brief"]').innerText().catch(() => "")));
const surfaceText = await p.locator('[data-testid="assistant-insights-surface"]').innerText();
check("H3 the figure chips carry last month + flagged count", /Last month in full/i.test(surfaceText) && /Flagged activity/i.test(surfaceText));

// --- attention pair states ---
check("A1 flagged tile shows findings or a designed all-clear (never vanishes)", (await p.locator('[data-testid="ast-all-clear"]').count()) === 1 || /Flagged activity/i.test(surfaceText));
check("A2 highlights tile shows shifts or its designed empty state", /Spending highlights/i.test(surfaceText));

// --- trend tile with takeaway ---
check("T1 the trend tile has a serif takeaway comparing to the six-month average", /six-month average/.test(await p.locator('[data-testid="ast-trend-takeaway"]').innerText().catch(() => "")));

// --- formulas: the toggle reveals the FormulaBuilder with the Assistant group ---
await p.locator('[data-testid="ast-toggle-formulas"]').click(); await p.waitForTimeout(900);
const afterToggle = await p.locator('[data-testid="assistant-insights-surface"]').innerText();
check("F1 the custom-values toggle reveals the formula tile", /formula variables \(assistant_/.test(afterToggle));
check("F2 the picker offers the ASSISTANT variable group", /ASSISTANT/.test(afterToggle));
await p.locator('[data-testid="ast-toggle-formulas"]').click(); await p.waitForTimeout(500);

// --- merchant drill-through lands on /transactions ---
const row = p.locator('[data-testid="assistant-insights-surface"] .insight-row--clickable').last();
if (await row.count()) {
  await row.click(); await p.waitForTimeout(900);
  check("D1 a merchant/highlight row drills through to /transactions", p.url().includes("/transactions"), p.url());
  await p.goto(URL + "/assistant", { waitUntil: "domcontentloaded" }); await p.waitForTimeout(1000);
} else {
  check("D1 a merchant/highlight row drills through to /transactions", false, "no clickable rows");
}

// --- empty dataset: the surface degrades to a single add-account CTA ---
await p.goto(URL + "/", { waitUntil: "domcontentloaded" }); await p.waitForTimeout(800);
const fresh = p.locator("text=Start fresh").first();
if (await fresh.count()) {
  await fresh.click(); await p.waitForTimeout(600);
  const confirm = p.locator("button", { hasText: /Start fresh|Confirm|Yes|Erase/ }).first();
  if (await confirm.count()) { await confirm.click().catch(() => {}); }
  await p.waitForTimeout(1500);
}
await p.goto(URL + "/assistant", { waitUntil: "domcontentloaded" }); await p.waitForTimeout(1000);
await p.locator(".seg-btn", { hasText: /^Insights$/ }).first().click(); await p.waitForTimeout(800);
const emptyText = await p.locator("main").innerText();
check("E1 with no accounts the briefing shows the add-account empty state", /briefing works best with real data|Add your first account/i.test(emptyText));

check("Z1 no page errors across the run", errs.length === 0, errs.slice(0, 3).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
