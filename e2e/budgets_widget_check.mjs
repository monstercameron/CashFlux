// e2e for the widgetized /budgets surface: it must mirror /accounts + /transactions —
// a `.bento.bento-budgets` surface host composing Native tiles (summary, toolbar,
// list, formula), preserving every feature, and tying in formulas + custom fields.
//
// Run: node e2e/budgets_widget_check.mjs  (against `go run e2e/serve.go <root> 8091`)
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8091";
const browser = await chromium.launch({ headless: true });
const results = []; let errs = [];
function check(n, c, d = "") { results.push({ n, ok: !!c }); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); }
async function ready(p) {
  await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
  await p.waitForFunction(() => { const b = document.getElementById("boot"); return !b || b.classList.contains("hidden") || b.offsetParent === null; }, { timeout: 15000 }).catch(() => {});
  await p.waitForTimeout(400);
}
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) errs.push(m); });

  // Load sample data so there are budgets to render.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" }); await ready(page);
  if (await page.locator('[data-testid="hero-load-sample"]').count()) {
    await page.locator('[data-testid="hero-load-sample"]').click(); await page.waitForTimeout(1800);
  }

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" }); await ready(page);
  await page.waitForTimeout(600);

  // B1 — the surface is the widgetized bento host (like /accounts + /transactions).
  check("B1 /budgets renders the .bento.bento-budgets surface host", await page.evaluate(() => !!document.querySelector(".bento.bento-budgets")));

  // B2 — the tiles compose as engine widgets (.w) inside the host.
  const tileCount = await page.evaluate(() => document.querySelectorAll(".bento.bento-budgets > .w").length);
  check("B2 surface composes engine tiles (.w)", tileCount >= 3, `tiles=${tileCount}`);

  // B3 — summary tile: the spent/budgeted/left stat grid.
  check("B3 summary tile shows the stat grid", await page.evaluate(() => !!document.querySelector(".bento-budgets .stat-grid")));

  // B4 — toolbar tile: method picker + 50/30/20 template + formulas toggle preserved.
  const tb = await page.evaluate(() => ({
    method: !!document.querySelector('[data-testid="budgets-method"]'),
    tmpl: !!document.querySelector('[data-testid="budgets-template-503020"]'),
    formulasToggle: !!document.querySelector('[data-testid="budgets-toggle-formulas"]'),
  }));
  check("B4 toolbar keeps method picker + 50/30/20 + formulas toggle", tb.method && tb.tmpl && tb.formulasToggle, JSON.stringify(tb));

  // B5 — list tile: budget rows render (sample data has budgets).
  const rows = await page.evaluate(() => document.querySelectorAll(".bento-budgets .budget").length);
  check("B5 list tile renders budget rows", rows > 0, `rows=${rows}`);

  // B6 — the budget-list section is titled (EntityListSection "Budgets").
  check("B6 list section is present", await page.evaluate(() => !!document.querySelector('.bento-budgets [data-testid="budget-list"], .bento-budgets .section, .bento-budgets .rows, .bento-budgets .budget')));

  // B7 — Formulas toggle reveals the FormulaBuilder tile (formulas + custom fields tie-in).
  const beforeFormula = await page.evaluate(() => document.body.innerText.includes("cf_budget_"));
  await page.locator('[data-testid="budgets-toggle-formulas"]').click(); await page.waitForTimeout(700);
  const afterFormula = await page.evaluate(() => document.body.innerText.includes("cf_budget_"));
  check("B7 Formulas toggle reveals the Budget metrics tile (cf_budget_ hint)", !beforeFormula && afterFormula, `before=${beforeFormula} after=${afterFormula}`);

  // B8 — switching the budgeting method still renders (in-context method switch works).
  await page.selectOption('[data-testid="budgets-method"]', "zero-based").catch(() => {});
  await page.waitForTimeout(600);
  check("B8 switching method keeps the surface rendered", await page.evaluate(() => !!document.querySelector(".bento.bento-budgets .stat-grid")));

  // B9 — Top up is a VISIBLE card button (the frequent action) and opens the flip modal.
  const topupVisible = await page.evaluate(() => {
    const btn = document.querySelector('.bento-budgets .budget-actions [data-testid^="budget-topup-btn-"]');
    return !!btn && !btn.closest(".add-menu"); // on the card, not inside the ⋯ menu
  });
  check("B9 Top up is a visible card button (not in the menu)", topupVisible);
  await page.locator('.bento-budgets .budget-actions [data-testid^="budget-topup-btn-"]').first().click().catch(() => {});
  await page.waitForTimeout(700);
  check("B9b Top up opens the flip modal with an amount field", await page.evaluate(() => !!document.getElementById("budget-topup-amt")));
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForFunction(() => !document.querySelector(".flip-backdrop.show"), { timeout: 5000 }).catch(() => {});
  await page.waitForTimeout(300);

  // B10 — the ⋯ overflow menu holds Edit + a destructive Delete (lower-frequency Edit
  // moved off the card; no standalone ✕), like /accounts.
  await page.locator(".bento-budgets .budget .add-wrap button[aria-haspopup='menu']").first().click();
  await page.waitForTimeout(400);
  const menu = await page.evaluate(() => ({
    open: !!document.querySelector(".bento-budgets .add-menu:not(.hidden-menu)"),
    edit: !!document.querySelector('.add-menu:not(.hidden-menu) [data-testid^="edit-budget-btn-"]'),
    del: !!document.querySelector('.add-menu:not(.hidden-menu) [data-testid^="delete-budget-btn-"]'),
    noStandaloneX: document.querySelectorAll(".bento-budgets .budget .btn-del").length === 0,
  }));
  check("B10 ⋯ menu holds Edit + Delete; no standalone ✕", menu.open && menu.edit && menu.del && menu.noStandaloneX, JSON.stringify(menu));

  // B11 — the menu's Edit opens the flip modal with the name field.
  await page.locator('.add-menu:not(.hidden-menu) [data-testid^="edit-budget-btn-"]').first().click();
  await page.waitForTimeout(700);
  check("B11 menu Edit opens the flip modal (name field present)", await page.evaluate(() => !!document.getElementById("budget-edit-name")));
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(300);

  // B12 — the "Transactions" button jumps to /transactions filtered by the category.
  check("B12 row has a Transactions review button", await page.evaluate(() => !!document.querySelector('.bento-budgets .budget-actions [data-testid^="budget-view-txns-"]')));
  await page.locator('.bento-budgets .budget-actions [data-testid^="budget-view-txns-"]').first().click().catch(() => {});
  await page.waitForTimeout(800);
  check("B12b clicking it navigates to /transactions", await page.evaluate(() => location.pathname.endsWith("/transactions")));

  const pass = results.filter(r => r.ok).length, fail = results.length - pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if (fail) { console.log("FAILED: " + results.filter(r => !r.ok).map(r => r.n).join(", ")); process.exitCode = 1; }
  console.log("page errors: " + (errs.length ? JSON.stringify([...new Set(errs)].slice(0, 6)) : "none"));
  console.log("════════════════════════════════════════════");
} finally { await browser.close(); }
