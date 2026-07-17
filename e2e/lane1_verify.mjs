// lane1_verify.mjs — end-to-end verification of the reports-trust lane fixes:
//   #47 / QA CF-01/UX-03 — a specific-account report scope actually narrows every
//        masthead figure, shows a plain-language "Showing X only" sentence that
//        stays visible with the Scope panel closed, and offers a one-click Reset.
// Usage: node e2e/lane1_verify.mjs   (server on :8111 serving the lane1 webroot)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8111";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));

const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1400); };

// Read the four masthead figures (label → value) from the reports hero.
const heroFigs = async () => {
  const figs = {};
  for (const f of await page.locator('[data-testid="rpt-hero"] .rpta-fig').all()) {
    const k = await f.locator(".rpta-fig-k").innerText();
    figs[k.trim()] = (await f.locator(".rpta-fig-v").innerText()).trim();
  }
  return figs;
};

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

// ───────────── #47: specific-account scope narrows the whole report ─────────────
await nav("/reports");
const all = await heroFigs();
check("#47 setup: masthead figures render household-wide", Object.keys(all).length >= 3, JSON.stringify(all));
check("#47 setup: no scope sentence while unscoped", (await page.locator('[data-testid="rpta-scope-line"]').count()) === 0);

// Open the Scope panel and tick exactly ONE account.
await page.locator('[data-testid="reports-scope-toggle"]').click();
await page.waitForTimeout(700);
await page.locator('[data-testid="scope-selector"] button:has-text("Specific accounts")').click();
await page.waitForTimeout(500);
const firstRow = page.locator(".scope-acct-row").first();
const acctName = (await firstRow.innerText()).trim();
await firstRow.locator('input[type="checkbox"]').check();
await page.waitForTimeout(1600);

const scoped = await heroFigs();
const changed = Object.keys(all).filter((k) => scoped[k] !== undefined && scoped[k] !== all[k]);
check("#47: masthead figures recalculate under a one-account scope", changed.length >= 2,
  `changed=[${changed.join(", ")}] all=${JSON.stringify(all)} scoped=${JSON.stringify(scoped)}`);

// The plain-language sentence + reset, visible even with the panel CLOSED.
await page.locator('[data-testid="reports-scope-toggle"]').click(); // close the panel
await page.waitForTimeout(700);
const line = page.locator('[data-testid="rpta-scope-line"]');
check("#47: scope sentence visible with the panel closed", (await line.count()) === 1);
if (await line.count()) {
  const txt = (await line.innerText()).replace(/\n/g, " ");
  check("#47: sentence names the scoped account in plain English",
    txt.includes("Showing") && txt.includes(acctName) && txt.includes("only"), txt);
  check("#47: sentence flags household-wide figures as unscoped", /household-wide/i.test(txt), txt);
}

// One-click reset restores the household view.
await page.locator('[data-testid="rpta-scope-reset"]').click();
await page.waitForTimeout(1600);
const resetFigs = await heroFigs();
const restored = Object.keys(all).every((k) => resetFigs[k] === all[k]);
check("#47: Reset scope restores every household figure", restored,
  `all=${JSON.stringify(all)} reset=${JSON.stringify(resetFigs)}`);
check("#47: scope sentence gone after reset", (await page.locator('[data-testid="rpta-scope-line"]').count()) === 0);

// ───────────── #46: snapshots inline + flip modals for report forms ─────────────
// The snapshot cluster renders INSIDE the toolbar row, beside its siblings.
const inRow = await page.locator('.rpta-toolbar-row [data-testid="reports-snapshots"]').count();
check("#46: snapshot cluster is inline in the toolbar row", inRow === 1);

// Take a snapshot, pick it: the frozen state opens in a flip MODAL, not inline.
await page.locator('[data-testid="reports-snap-take"]').click();
await page.waitForTimeout(900);
const snapSel = page.locator('[data-testid="reports-snap-select"]');
const snapVal = await snapSel.locator("option").nth(1).getAttribute("value");
await snapSel.selectOption(snapVal);
await page.waitForTimeout(900); // FlipPanel's 550ms flip
const snapInModal = await page.locator('.flip-panel [data-testid="report-snap-panel"], .flip-wrap [data-testid="report-snap-panel"]').count();
check("#46: frozen snapshot opens in a flip modal", snapInModal >= 1);
await page.locator(".set-btn.close").click();
await page.waitForTimeout(900);
check("#46: snapshot modal closes", (await page.locator('[data-testid="report-snap-panel"]').count()) === 0);

// Report metrics: the builder opens in a flip modal, not an inline appendix expander.
await page.locator('[data-testid="reports-toggle-formulas"]').click();
await page.waitForTimeout(900);
const metricsInModal = await page.locator('.flip-panel [data-testid="reports-metrics-modal"], .flip-wrap [data-testid="reports-metrics-modal"]').count();
check("#46: metrics builder opens in a flip modal", metricsInModal >= 1);
const metricsInAppendix = await page.locator('#rpta-11 [data-testid="reports-metrics-modal"], .rpta-section [data-testid="reports-metrics-modal"]').count();
check("#46: metrics builder no longer expands inline in the appendix", metricsInAppendix === 0);
await page.locator(".set-btn.close").click();
await page.waitForTimeout(900);
check("#46: metrics modal closes", (await page.locator('[data-testid="reports-metrics-modal"]').count()) === 0);

// Saved views: "Save view" opens a flip modal with the standard Save footer.
await page.locator('[data-testid="reports-saved-open"]').click();
await page.waitForTimeout(900);
const savedNameInModal = await page.locator('.flip-panel [data-testid="reports-saved-name"], .flip-wrap [data-testid="reports-saved-name"]').count();
check("#46: save-view name form opens in a flip modal", savedNameInModal >= 1);
await page.locator('[data-testid="reports-saved-name"]').fill("Lane1 QA view");
await page.locator('[data-testid="reports-saved-confirm"]').click();
await page.waitForTimeout(1200);
check("#46: saving closes the modal", (await page.locator('[data-testid="reports-saved-name"]').count()) === 0);
const savedOpts = await page.locator('[data-testid="reports-saved-select"] option').allInnerTexts().catch(() => []);
check("#46: the named view lands in the picker", savedOpts.some((t) => t.includes("Lane1 QA view")), savedOpts.join(","));

// Scope panel: "Save view" (saved scope views) also opens a flip modal.
await page.locator('[data-testid="reports-scope-toggle"]').click();
await page.waitForTimeout(700);
await page.locator('[data-testid="scope-selector"] .scope-chip').first().click(); // make the saved-views row appear
await page.waitForTimeout(700);
const scopeSaveBtn = page.locator('[data-testid="scope-selector"] button', { hasText: "Save current as" }).first();
if (await scopeSaveBtn.count()) {
  await scopeSaveBtn.click();
  await page.waitForTimeout(900);
  const scopeFormInModal = await page.locator('.flip-panel #scope-save-form, .flip-wrap #scope-save-form').count();
  check("#46: scope saved-view form opens in a flip modal", scopeFormInModal >= 1);
  await page.locator(".set-btn.cancel, .set-close").first().click();
  await page.waitForTimeout(900);
} else {
  check("#46: scope saved-view save button reachable", false);
}
// reset the chip we toggled
await page.locator('[data-testid="scope-selector"] .scope-chip').first().click();
await page.waitForTimeout(600);
await page.locator('[data-testid="reports-scope-toggle"]').click();
await page.waitForTimeout(500);

// ───────────── #54: financial audit trail — before/after, cause, downstream ─────────────
// Make one manual edit (toggle "Exclude from reports" on the first transaction
// via its kebab), then read the audit trail: the entry must show a field-level
// before → after diff, the manual actor, and the downstream figures recalculated.
await nav("/transactions");
// Use a fresh row (nth 3) and accept either direction of the toggle — repeated
// runs against the same origin flip it back and forth.
const targetRow = page.locator("table tbody tr").nth(3);
await targetRow.hover(); // row actions are hover-revealed
await page.waitForTimeout(300);
await targetRow.locator('button[title="Transaction actions"], button[aria-label="Transaction actions"]').first().click({ force: true });
await page.waitForTimeout(600);
await page.locator('button:visible', { hasText: /Exclude from reports|Include in reports/ }).first().click();
await page.waitForTimeout(1600);

await nav("/activity");
// Async KV autosaves and sample-seeded history share the feed — assert on OUR
// entry: the one whose diff names the exclude-from-reports field.
const firstEntry = page.locator(".row.act-entry", { hasText: "exclude from reports" }).first();
check("#54: audit trail has entries", (await page.locator(".row.act-entry").count()) > 0);
const entryText = (await firstEntry.innerText().catch(() => "")).replace(/\n/g, " ");
const diffCount = await firstEntry.locator(".act-diff-line").count();
check("#54: newest entry carries a field-level before → after diff", diffCount >= 1, entryText.slice(0, 200));
if (diffCount) {
  const before = await firstEntry.locator(".act-diff-before").first().innerText();
  const after = await firstEntry.locator(".act-diff-after").first().innerText();
  check("#54: diff shows distinct before and after values", before !== "" && after !== "" && before !== after, `${before} -> ${after}`);
}
check("#54: entry names its downstream recalculated figures",
  (await firstEntry.locator('[data-testid="act-recalc"]').count()) === 1 && /Recalculated:/.test(entryText), entryText.slice(0, 220));
check("#54: manual edit attributed to You", (await firstEntry.locator('[data-testid="act-actor"]').innerText().catch(() => "")).includes("You"));

// Rules cause: create a rule that genuinely recategorizes an existing payee,
// run the backfill, and check the audit entry is attributed to "Rule".
await nav("/transactions");
// A rule matches on payee/description CONTAINS — pull the first alphabetic word
// (≥4 letters) from a clean data row, skipping UI badge words.
const payeeWord = await page.evaluate(() => {
  const stop = new Set(["EXCLUDED", "Split", "UPCOMING", "Edit", "Cleared", "Reviewed"]);
  for (const row of document.querySelectorAll("table tbody tr")) {
    const text = row.innerText || "";
    // ApplyRules skips transfers and liability payments look-alikes — pick a
    // word from a plain spending row.
    if (/Transfer|payment|Payment/i.test(text)) continue;
    for (const m of text.matchAll(/[A-Za-z]{4,}/g)) {
      if (!stop.has(m[0])) return m[0];
    }
  }
  return "";
});
await nav("/rules");
const ruleForm = page.locator('[data-testid="rule-add-form"]').first();
if ((await ruleForm.count()) && payeeWord.length >= 3) {
  await ruleForm.locator('input[type="text"]').first().fill(payeeWord);
  const catSel = ruleForm.locator("select").first();
  const catOpts = await catSel.locator("option").all();
  for (let i = catOpts.length - 1; i > 0; i--) { // last real category → guaranteed different for most rows
    const v = await catOpts[i].getAttribute("value");
    if (v) { await catSel.selectOption(v); break; }
  }
  await ruleForm.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1200);
  const applyBtn = page.locator('button:has-text("Apply to existing")').first();
  if (await applyBtn.count()) {
    await applyBtn.click();
    await page.waitForTimeout(800);
    // The backfill previews its blast radius behind a confirm dialog.
    const confirmBtn = page.locator("#cf-dialog-confirm");
    if (await confirmBtn.count()) await confirmBtn.click();
    await page.waitForTimeout(1800);
    await nav("/activity");
    const ruleBadges = await page.locator('[data-testid="act-actor"]', { hasText: "Rule" }).count();
    check("#54: rules backfill attributed to Rule", ruleBadges >= 1, `ruleBadges=${ruleBadges} word=${payeeWord}`);
  } else {
    check("#54: rules apply button reachable", false, "no 'Apply to existing' on /rules");
  }
} else {
  check("#54: rule creation preconditions", false, `form=${await ruleForm.count()} word=${payeeWord}`);
}

check("no page errors", errors.length === 0, errors.slice(0, 3).join(" | "));

await browser.close();
console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
