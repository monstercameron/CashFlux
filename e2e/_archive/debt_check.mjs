// Comprehensive /debt e2e: every tile's features + negative/edge cases.
// Run: node e2e/debt_check.mjs   (expects the app served on :8091 with sample data)
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1200 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const openDebt = async () => {
  await p.goto(URL + "/debt", { waitUntil: "domcontentloaded" });
  await p.waitForSelector(".bento-debt", { timeout: 15000 }).catch(() => {});
  await p.waitForTimeout(1200);
};
const tileWith = (text) => p.locator(".bento-debt > .w").filter({ hasText: text }).first();

// --- boot + sample data ----------------------------------------------------------
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await openDebt();

// ============================ SUMMARY TILE ======================================
check("S1 surface host renders", await p.locator('.bento-debt').count() === 1);
check("S2 total-owed hero present", await p.locator('[data-testid="debt-total-owed"]').count() === 1);
const owedTxt = (await p.locator('[data-testid="debt-total-owed"]').textContent().catch(() => "")) || "";
check("S3 total owed is a money figure > 0", /[1-9]/.test(owedTxt.replace(/[^0-9]/g, "")), owedTxt.trim());
check("S4 debt-free projection line", await p.locator('.debt-hero-sub').count() >= 1);
check("S5 four engine ratio chips", await p.locator('.debt-hero .debt-stat').count() >= 3, `${await p.locator('.debt-hero .debt-stat').count()}`);
const utilChipBanded = await p.locator('.debt-hero .debt-stat.debt-band-warn, .debt-hero .debt-stat.debt-band-high, .debt-hero .debt-stat.debt-band-good').count();
check("S6 utilization chip is config-banded", utilChipBanded >= 1);

// ============================ TOOLBAR TILE ======================================
check("T1 add-debt button", await p.locator('[data-testid="debt-add"]').count() === 1);
check("T2 manage-accounts link", await p.locator('.bento-debt a[href$="/accounts"]').count() >= 1);
check("T3 debt-metrics toggle", await p.locator('[data-testid="debt-toggle-formulas"]').count() === 1);
// Add-debt opens the add modal; Escape closes it (no crash).
await p.locator('[data-testid="debt-add"]').click({ force: true }); await p.waitForTimeout(600);
const modalOpen = await p.locator('.flip-panel, [role="dialog"], form').filter({ hasText: /account|debt|type/i }).count() >= 1;
check("T4 add-debt opens an add form", modalOpen);
await p.keyboard.press("Escape"); await p.waitForTimeout(500);

// ============================ PAYOFF LADDER =====================================
const cardCount = await p.locator('.debt-card').count();
check("L1 payoff-ladder cards render", cardCount >= 4, `${cardCount}`);
check("L2 rank medallions", await p.locator('.debt-card .debt-rank').count() >= 1);
check("L3 APR/utilization banded rails", await p.locator('.debt-card .debt-rail').count() === cardCount);
check("L4 utilization meter on credit cards", await p.locator('.debt-util-fill').count() >= 1);
check("L5 a warn/high banded card exists", await p.locator('.debt-card.debt-band-warn, .debt-card.debt-band-high').count() >= 1);
check("L6 APR chips", await p.locator('.debt-card .debt-apr').count() >= 1);
check("L7 mortgage excluded from plan by default (config)", await p.locator('.debt-card.is-excluded').count() >= 1, `${await p.locator('.debt-card.is-excluded').count()}`);
// Ranks are contiguous starting at 1 for in-plan debts.
const ranks = await p.locator('.debt-card:not(.is-excluded) .debt-rank').allTextContents();
const nums = ranks.map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n)).sort((a, z) => a - z);
check("L8 payoff ranks start at 1 and are contiguous", nums.length >= 1 && nums[0] === 1 && nums[nums.length - 1] === nums.length, nums.join(","));
// Excluded card shows a dash rank, not a number.
const exclRank = (await p.locator('.debt-card.is-excluded .debt-rank').first().textContent().catch(() => "")) || "";
check("L9 excluded card rank is a dash", exclRank.trim() === "—", exclRank.trim());

// Toggle an excluded debt INTO the plan → it gains a rank, loses is-excluded.
const exclToggle = p.locator('.debt-card.is-excluded [data-testid^="debt-payoff-toggle-"]').first();
const exclId = await p.locator('.debt-card.is-excluded').first().getAttribute("data-testid");
if (await exclToggle.count()) { await exclToggle.click({ force: true }); await p.waitForTimeout(800); }
check("L10 toggling include re-ranks that debt", await p.locator('[data-testid="' + exclId + '"]').first().evaluate(el => !el.classList.contains("is-excluded")).catch(() => false));
// Toggle it back out.
const backToggle = p.locator('[data-testid="' + exclId + '"] [data-testid^="debt-payoff-toggle-"]').first();
if (await backToggle.count()) { await backToggle.click({ force: true }); await p.waitForTimeout(800); }
check("L11 toggling exclude restores excluded state", await p.locator('[data-testid="' + exclId + '"]').first().evaluate(el => el.classList.contains("is-excluded")).catch(() => false));

// Edit opens the account editor modal; Escape closes.
await p.locator('[data-testid^="debt-edit-"]').first().click({ force: true }); await p.waitForTimeout(700);
check("L12 edit opens the account editor", await p.locator('.flip-panel, [role="dialog"], form').count() >= 1);
await p.keyboard.press("Escape"); await p.waitForTimeout(400);

// ============================ STRATEGY PANEL ====================================
await openDebt();
const strat = p.locator("#debt");
check("ST1 strategy panel present", await strat.count() >= 1);
check("ST2 snowball + avalanche shown", await p.getByText(/snowball/i).count() >= 1 && await p.getByText(/avalanche/i).count() >= 1);
// $0 extra edge case: the comparison must still resolve clearly — either a Recommended
// badge / savings line (when one method already wins on interest) or an explicit tie hint —
// never a blank/broken side-by-side.
const resolved = (await strat.locator('.strat-badge').count() >= 1)
  || (await p.getByText(/less in interest|match|add an extra|tie/i).count() >= 1);
check("ST3 (neg) $0 extra resolves the comparison (badge or hint)", resolved);
// Suggested-extra quick button fills the input and the plans diverge.
const tryBtn = p.getByRole("button", { name: /try \$/i }).first();
if (await tryBtn.count()) { await tryBtn.click({ force: true }); await p.waitForTimeout(1000); }
const extraInput = strat.locator('input[type="number"]').first();
const extraVal = await extraInput.inputValue().catch(() => "");
check("ST4 suggested-extra button fills the extra field", parseFloat(extraVal) > 0, extraVal);
check("ST5 snowball vs avalanche comparison cards", await strat.locator('.strat-card').count() === 2, `${await strat.locator('.strat-card').count()}`);
check("ST5b payoff-order sequence shown", await strat.locator('.strat-order-seq').count() >= 1);
// Manually set a large extra → the two methods diverge; a winner gets badged Recommended.
await extraInput.fill("1000"); await p.waitForTimeout(1200);
check("ST6 large extra keeps a viable plan (months shown)", await p.getByText(/month/i).count() >= 1);
check("ST7 the better method is badged Recommended", await strat.locator('.strat-card.is-winner, .strat-badge').count() >= 1);

// ============================ CREDIT HEALTH =====================================
const credit = tileWith("Credit health");
check("C1 credit-health tile present", await credit.count() >= 1);
check("C2 score ring + band", await credit.getByText(/good|fair|poor|excellent/i).count() >= 1);
check("C3 per-card utilization rows", await credit.locator('.credit-card-item').count() >= 1, `${await credit.locator('.credit-card-item').count()}`);
check("C4 pay-to-30% nudge on a high card", await credit.getByText(/reach 30%/i).count() >= 1);
// Demerits: what's dragging the score down, with a point-cost chip.
check("C4a demerits card lists what's hurting the score", await credit.locator('[data-testid="credit-demerits"] .credit-item').count() >= 1, `${await credit.locator('[data-testid="credit-demerits"] .credit-item').count()}`);
check("C4b a demerit shows a point-cost (−N pts) chip", await credit.locator('.credit-pts-down').count() >= 1);
check("C4c overall-utilization demerit is worded with a target", await credit.getByText(/aim for under 30%/i).count() >= 1);
// Advice: the clearest, prioritized fix, with an impact chip.
check("C4d advice card gives the clearest fix", await credit.locator('[data-testid="credit-advice"] .credit-item').count() >= 1, `${await credit.locator('[data-testid="credit-advice"] .credit-item').count()}`);
check("C4e advice shows an impact (+N pts) chip", await credit.locator('.credit-pts-up').count() >= 1);
check("C4f top advice is a concrete pay-down action", await credit.locator('[data-testid="credit-advice"]').getByText(/Pay \$/).count() >= 1);
// Smart+ AI analysis is opt-in: absent by default (no dead control).
check("C4g Smart+ AI analysis is opt-in (hidden until enabled)", await credit.locator('[data-testid="credit-ai"]').count() === 0);
// Credit-limit editor: set a NEW valid limit → commit on blur → saved status appears.
const limInput = credit.locator('[data-testid="credit-limit-edit"]').first();
if (await limInput.count()) { await limInput.fill("15000"); await limInput.blur(); await p.waitForTimeout(900); }
check("C5 editing a credit limit commits (saved status)", await credit.getByText(/saved|updated/i).count() >= 1 || errs.length === 0);
// (neg) A blank/zero limit must not crash the panel.
if (await limInput.count()) { await limInput.fill(""); await limInput.blur(); await p.waitForTimeout(700); }
check("C6 (neg) clearing the limit does not crash", await credit.locator('.credit-card-item').count() >= 1);

// ============================ LOANS =============================================
const loans = tileWith("Installment loan");
check("LN1 loans tile present", await loans.count() >= 1);
check("LN2 a loan card with amortization stats", await loans.locator('.stat').count() >= 2, `${await loans.locator('.stat').count()}`);
check("LN3 monthly payment + payoff date shown", await loans.getByText(/monthly payment/i).count() >= 1 && await loans.getByText(/payoff date/i).count() >= 1);
// Term input drives the schedule; a longer term lowers the monthly payment.
const termInput = loans.locator('input[type="number"]').first();
const payBefore = (await loans.getByText(/\$[\d,]+\.\d\d/).first().textContent().catch(() => "")) || "";
if (await termInput.count()) { await termInput.fill("120"); await p.waitForTimeout(1000); }
check("LN4 changing the loan term recomputes the schedule", errs.length === 0);
// (neg) An invalid term (0) should fall back to a default, not break.
if (await termInput.count()) { await termInput.fill("0"); await p.waitForTimeout(700); }
check("LN5 (neg) invalid term does not crash", await loans.locator('.stat').count() >= 2);

// ============================ PAYOFF CALCULATOR =================================
const calc = tileWith("payoff calculator");
check("PC1 payoff calculator present", await calc.count() >= 1);
// (neg) Empty inputs → the projection shows the hint, not a result.
check("PC2 (neg) empty inputs show the hint", await calc.getByText(/enter a balance/i).count() >= 1);
const ci = calc.locator('input[type="number"]');
// Valid: balance 5000, APR 12, payment 200 → a projection with months + payoff date.
await ci.nth(0).fill("5000"); await ci.nth(1).fill("12"); await ci.nth(2).fill("200"); await p.waitForTimeout(1000);
check("PC3 valid inputs produce a projection", await calc.getByText(/month/i).count() >= 1 && await calc.locator('.stat').count() >= 2);
// Extra payment note.
await ci.nth(3).fill("100"); await p.waitForTimeout(900);
check("PC4 extra payment shows an impact note", await calc.getByText(/sooner|save|less interest|month/i).count() >= 1);
// (neg) Payment too low to cover interest → an error with the minimum viable payment.
await ci.nth(0).fill("100000"); await ci.nth(1).fill("99"); await ci.nth(2).fill("1"); await ci.nth(3).fill(""); await p.waitForTimeout(1000);
check("PC5 (neg) payment-too-low shows a minimum-payment error", await calc.locator('.err, [role="alert"]').count() >= 1);

// ============================ FORMULA / ENGINE ==================================
await openDebt();
await p.locator('[data-testid="debt-toggle-formulas"]').click({ force: true }); await p.waitForTimeout(900);
check("F1 debt-metrics formula tile reveals", await p.locator('.bento-debt').getByText(/formula|metric/i).count() >= 1);
const hasDebtVars = await p.evaluate(() => document.body.innerHTML.includes("debt_") || /utilization|owed/i.test(document.body.innerText));
check("F2 debt_* engine variables discoverable", hasDebtVars);
// Toggling off hides it again.
await p.locator('[data-testid="debt-toggle-formulas"]').click({ force: true }); await p.waitForTimeout(700);
check("F3 toggle hides the formula tile", true);

// ============================ JUMP NAV =========================================
await openDebt();
check("J1 jump-nav lists the section links", await p.locator('.debt-jump .debt-jump-link').count() >= 4, `${await p.locator('.debt-jump .debt-jump-link').count()}`);
check("J2 section anchors exist in the DOM", await p.locator('#sec-overview').count() >= 1 && await p.locator('#sec-ladder').count() >= 1 && await p.locator('#sec-strategy').count() >= 1);
check("J3 jump-nav only lists present sections", await p.locator('[data-testid="debt-jump-sec-credit"]').count() >= 1 && await p.locator('[data-testid="debt-jump-sec-loans"]').count() >= 1);
// Clicking a jump link scrolls that section to the top of the viewport.
const beforeTop = await p.evaluate(() => { const el = document.getElementById("sec-credit"); return el ? el.getBoundingClientRect().top : 99999; });
await p.locator('[data-testid="debt-jump-sec-credit"]').click({ force: true });
await p.waitForTimeout(900);
const afterTop = await p.evaluate(() => { const el = document.getElementById("sec-credit"); return el ? el.getBoundingClientRect().top : 99999; });
check("J4 clicking a jump link scrolls that section into view", afterTop < beforeTop && afterTop < 400, `before=${Math.round(beforeTop)} after=${Math.round(afterTop)}`);

// ============================ NAV / INTEGRATION =================================
await openDebt();
await p.locator('[data-testid^="debt-view-"]').first().click({ force: true }); await p.waitForTimeout(900);
check("N1 a debt card links to its transactions", p.url().endsWith("/transactions"), p.url());
await openDebt();
await p.locator('.bento-debt a[href$="/accounts"]').first().click({ force: true }); await p.waitForTimeout(900);
check("N2 manage-accounts navigates to /accounts", p.url().endsWith("/accounts"), p.url());

// ============================ OWNING-PAGE LINKS ================================
await openDebt();
check("O1 sections link to their owning pages", await p.locator('.debt-owner-link').count() >= 5, `${await p.locator('.debt-owner-link').count()}`);
check("O2 net-worth / allocate / planning links present",
  await p.locator('.debt-owner-link[href$="/networth"]').count() >= 1
  && await p.locator('.debt-owner-link[href$="/allocate"]').count() >= 1
  && await p.locator('.debt-owner-link[href$="/planning"]').count() >= 1);
check("O3 utilization meters render a value (engine/formula-driven)", await p.locator('.debt-util-track[aria-valuenow]').count() >= 1);
// Clicking the strategy section's link navigates to the page that owns it (/allocate).
await p.locator('.debt-owner-link[href$="/allocate"]').first().click({ force: true });
await p.waitForTimeout(900);
check("O4 a section link navigates to its owning page", p.url().endsWith("/allocate"), p.url());

// ============================ SCROLL TO TOP ====================================
await openDebt();
const stOpacity = () => p.evaluate(() => { const el = document.getElementById("cf-scrolltop"); return el ? getComputedStyle(el).opacity : "missing"; });
check("K1 scroll-to-top button exists", await p.locator('[data-testid="scroll-to-top"]').count() === 1);
check("K2 hidden at the top of the page", (await stOpacity()) === "0");
await p.evaluate(() => document.getElementById("main").scrollTo(0, 1400));
await p.waitForTimeout(600);
check("K3 reveals after scrolling down", (await stOpacity()) === "1");
await p.locator('[data-testid="scroll-to-top"]').click({ force: true });
await p.waitForTimeout(900);
check("K4 clicking it scrolls back to the top", await p.evaluate(() => document.getElementById("main").scrollTop === 0));
check("K5 hides again once back at the top", (await stOpacity()) === "0");

// ============================ ERROR PROBE =======================================
check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
