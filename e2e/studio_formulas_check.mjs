// Studio → Formulas tab comprehensive e2e: the searchable variable palette
// (derived groups incl. the previously-hidden Assistant group, collapsible with
// counts), the workbench (insert → live result → save), and the compound-
// variable editor — the addressability keystone: editing health_score here
// reshapes /health live, and reset restores the built-in. Positive AND
// negative cases (invalid formula rejected with the error shown, taken name
// rejected, search miss shows a calm empty note). No page errors.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 2000 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const openFormulasTab = async () => {
  await p.goto(URL + "/studio", { waitUntil: "domcontentloaded" });
  await p.waitForTimeout(1500);
  await p.locator('button, [role="radio"]', { hasText: "Formulas" }).first().click({ force: true });
  await p.waitForSelector(".bento-studio", { timeout: 15000 }).catch(() => {});
  await p.waitForTimeout(1200);
};

// boot + sample
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await openFormulasTab();

// --- palette: derived collapsible groups + search ---
check("S1 the formulas tab is a bento surface", await p.locator(".bento-studio").count() === 1);
check("P1 groups collapse behind headers with counts", await p.locator(".fb-pal-head").count() >= 10, `${await p.locator(".fb-pal-head").count()} groups`);
check("P2 the derived groups include the once-hidden Assistant group", await p.locator('[data-testid="fb-group-Assistant"]').count() === 1);
const openGrids = await p.locator(".fb-pal-grid").count();
check("P3 only the first group starts open (no wall of chips)", openGrids === 1, `${openGrids} open grids`);
await p.locator('[data-testid="fb-group-Health factors"]').click({ force: true });
await p.waitForTimeout(300);
check("P4 clicking a group header expands it", await p.locator(".fb-pal-grid").count() === 2);
// Search filters to a flat grid; a miss shows a calm note.
await p.locator('[data-testid="fb-search"]').fill("runway");
await p.waitForTimeout(500);
check("P5 search flattens to matches", /matches/i.test(await p.locator(".fb-palette").innerText()) && (await p.locator(".fb-pal-head").count()) === 0);
await p.locator('[data-testid="fb-search"]').fill("zzzznope");
await p.waitForTimeout(500);
check("P6 (neg) a search miss shows a calm empty note", /Nothing matches/.test(await p.locator(".fb-palette").innerText()));
await p.locator('[data-testid="fb-search"]').fill("");
await p.waitForTimeout(400);

// --- workbench: insert → result → save ---
await p.locator(".fb-pal-grid .fb-chip").first().click({ force: true });
await p.waitForTimeout(400);
const exprVal = await p.locator(".fb-expr").inputValue();
check("W1 clicking a chip inserts its variable", exprVal.trim().length > 0, exprVal);
check("W2 the live result evaluates", /[\d—]/.test(await p.locator(".fb-result").innerText()));

// --- compound variables: the addressability round-trip ---
check("M1 the compound-variables tile lists molecules with live values + formulas", await p.locator('[data-testid^="mol-row-"]').count() >= 6 && (await p.locator("#sec-stu-molecules .stu-mol-formula").count()) >= 6);
// Edit health_score to a trivially different formula, save, verify /health follows.
await p.locator('[data-testid="mol-edit-health_score"]').click({ force: true });
await p.waitForTimeout(400);
await p.locator('[data-testid="mol-draft-health_score"]').fill("clamp(health_savings, 0, 100)");
await p.waitForTimeout(500);
check("M2 the editor previews the draft live", /=/.test((await p.locator('[data-testid="mol-preview-health_score"]').innerText().catch(() => "")) || ""));
await p.locator('[data-testid="mol-save-health_score"]').click({ force: true });
await p.waitForTimeout(800);
check("M3 saving tags the molecule as edited", /edited/.test(await p.locator('[data-testid="mol-row-health_score"]').innerText()));
// The /health hero renders THE EDITED formula — the page reads the molecule.
// (Full navigation = full reload; wait out the ~4s dataset autosave ticker so
// the persisted override survives the reload.)
await p.waitForTimeout(4600);
await p.goto(URL + "/health", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".bento-health", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1500);
await p.locator('[data-testid="health-formula"] summary').click({ force: true }).catch(() => {});
await p.waitForTimeout(300);
const healthFormula = (await p.locator('[data-testid="health-formula"] code').innerText().catch(() => "")) || "";
check("M4 /health now shows the EDITED definition (the page reads the molecule)", healthFormula.includes("clamp(health_savings, 0, 100)"), healthFormula.slice(0, 60));
// Reset restores the built-in.
await openFormulasTab();
await p.locator('[data-testid="mol-edit-health_score"]').click({ force: true });
await p.waitForTimeout(400);
await p.locator('[data-testid="mol-reset-health_score"]').click({ force: true });
await p.waitForTimeout(800);
const rowTxt = await p.locator('[data-testid="mol-row-health_score"]').innerText();
check("M5 reset restores the built-in definition", /built-in/.test(rowTxt) && !/edited/.test(rowTxt));

// --- negatives: invalid formula + taken name ---
await p.locator('[data-testid="mol-edit-health_score"]').click({ force: true });
await p.waitForTimeout(300);
await p.locator('[data-testid="mol-draft-health_score"]').fill("1 + (");
await p.waitForTimeout(400);
check("N1 (neg) an invalid draft previews as an error", /doesn't evaluate/.test(await p.locator('[data-testid="mol-row-health_score"]').innerText()));
await p.locator('[data-testid="mol-save-health_score"]').click({ force: true });
await p.waitForTimeout(500);
check("N2 (neg) saving an invalid formula is rejected with the error shown", /invalid formula|doesn't evaluate/.test(await p.locator('[data-testid="mol-row-health_score"]').innerText()));
await p.locator('[data-testid="mol-cancel-health_score"]').click({ force: true });
await p.waitForTimeout(300);
// New-variable form: a taken name is rejected.
await p.locator('[data-testid="mol-new-name"]').fill("net_worth");
await p.locator('[data-testid="mol-new-formula"]').fill("assets * 2");
await p.locator('[data-testid="mol-new-create"]').click({ force: true });
await p.waitForTimeout(400);
check("N3 (neg) creating a variable with a taken name is rejected", /already exists/.test(await p.locator('[data-testid="mol-new-form"]').innerText()));
// A valid custom variable creates, appears tagged "yours", and deletes.
await p.locator('[data-testid="mol-new-name"]').fill("fun_money");
await p.locator('[data-testid="mol-new-formula"]').fill("safe_to_spend * 0.1");
await p.locator('[data-testid="mol-new-create"]').click({ force: true });
await p.waitForTimeout(800);
check("M6 a new custom variable appears tagged yours", /yours/.test((await p.locator('[data-testid="mol-row-fun_money"]').innerText().catch(() => "")) || ""));
await p.locator('[data-testid="mol-edit-fun_money"]').click({ force: true });
await p.waitForTimeout(300);
await p.locator('[data-testid="mol-delete-fun_money"]').click({ force: true });
await p.waitForTimeout(800);
check("M7 deleting a custom variable removes it", (await p.locator('[data-testid="mol-row-fun_money"]').count()) === 0);

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
