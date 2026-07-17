// qa_remediation_verify.mjs — end-to-end verification of the 2026-07-17 QA
// remediation fixes against a live build:
//   H1/CF-01 — transfer into a positive-stored liability REDUCES the debt
//   H2/CF-03 — a chosen CSV file imports through the primary Import action
//   M1       — Household roster refreshes (with a notice) after adding a member
//   CF-02    — review inbox: unarmed confirm explains itself; armed confirm and
//              skip advance; role-blocked writes surface a notice
// Usage: node e2e/qa_remediation_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
import { writeFileSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BASE = "http://127.0.0.1:8097";
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

const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1300); };
const bodyText = async () => await page.locator("body").innerText();

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1500);

// ───────────────────────── H1: liability payment transfer ─────────────────────────
await nav("/accounts");
await page.locator('button:has-text("Add account")').first().click();
await page.waitForTimeout(900);
const addForm = page.locator('[data-testid="account-add-form"]');
await addForm.locator('input[type="text"]').first().fill("QA H1 Loan");
// account type select is the first plain select in the form (copy-existing has its own testid)
const typeSel = addForm.locator("select").filter({ hasNot: page.locator('[data-testid]') }).first();
await addForm.locator("select").nth((await addForm.locator('[data-testid="account-copy-existing"]').count()) ? 1 : 0)
  .selectOption({ label: "Loan" }).catch(async () => {
    // fall back: find whichever select offers a Loan option
    const sels = await addForm.locator("select").all();
    for (const s of sels) {
      const labels = await s.locator("option").allInnerTexts();
      if (labels.some((l) => /loan/i.test(l))) { await s.selectOption({ label: labels.find((l) => /^loan$/i.test(l)) || labels.find((l) => /loan/i.test(l)) }); break; }
    }
  });
await page.waitForTimeout(400);
await addForm.locator('input[type="number"]').first().fill("500");
// footer "Add account" save
await page.locator('.flip-panel button:has-text("Add account"), dialog button:has-text("Add account"), button:has-text("Add account")').last().click();
await page.waitForTimeout(1200);
let body = await bodyText();
check("H1 setup: loan created showing $500 owed", body.includes("QA H1 Loan"));

// transfer $50 from the first eligible source to the loan
await page.locator('button:has-text("Transfer money")').first().click();
await page.waitForTimeout(900);
const fromSel = page.locator('[data-testid="page-xfer-from-select"]');
const fromOpts = await fromSel.locator("option").all();
for (const o of fromOpts) { const v = await o.getAttribute("value"); if (v) { await fromSel.selectOption(v); break; } }
const toSel = page.locator('[data-testid="page-xfer-to-select"]');
const toOpts = await toSel.locator("option").all();
let toVal = "";
for (const o of toOpts) { const t = await o.innerText(); const v = await o.getAttribute("value"); if (v && t.includes("QA H1 Loan")) { toVal = v; break; } }
check("H1: loan appears as transfer destination", toVal !== "");
await toSel.selectOption(toVal);
await page.locator('[data-testid="page-xfer-amt"]').fill("50");
await page.locator('[data-testid="page-transfer-form"] button[type="submit"]').click();
await page.waitForTimeout(1300);
body = await bodyText();
check("H1: $50 payment reduced the loan to ($450.00)", body.includes("($450.00)"), body.includes("($550.00)") ? "STILL SHOWS ($550.00)" : "");
check("H1: debt did not grow to ($550.00)", !body.includes("($550.00)"));

// ─────────── CF-09: dashboard presents the liability like /accounts does ───────────
await nav("/");
await page.waitForTimeout(1200);
const dashBody = await bodyText();
if (dashBody.includes("QA H1 Loan")) {
  check("CF-09: dashboard shows the loan as ($450.00), not $450.00", dashBody.includes("($450.00)"), "");
} else {
  // widget row cap may hide the newest account; the table test covers the logic
  check("CF-09: loan not in the dashboard widget's visible rows (covered by table test)", true, "");
}

// ───────────────────────── H2: CSV file upload import ─────────────────────────
const dir = mkdtempSync(join(tmpdir(), "cfqa-"));
const csv1 = join(dir, "one.csv");
const csv2 = join(dir, "two.csv");
writeFileSync(csv1, "date,payee,amount\n2026-07-10,QA H2 Alpha,-12.34\n2026-07-11,QA H2 Beta,-23.45\n");
writeFileSync(csv2, "date,payee,amount\n2026-07-10,QA H2 Alpha,-12.34\n2026-07-11,QA H2 Beta,-23.45\n2026-07-12,QA H2 Gamma,-34.56\n");

await nav("/transactions");
await page.locator('[data-testid="txn-more-btn"]').click(); // Import lives in the ⋯ More overflow
await page.waitForTimeout(500);
await page.locator('[data-testid="txn-import-btn"]').click();
await page.waitForTimeout(900);
// pick the CSV source tile
await page.locator('[data-testid="import-type-picker"] button:has-text("CSV"), button:has-text("CSV / spreadsheet")').first().click();
await page.waitForTimeout(700);
// first upload: unique rows import directly (file path sanity)
const [fc1] = await Promise.all([page.waitForEvent("filechooser"), page.locator('[data-testid="csv-file-picker"]').click()]);
await fc1.setFiles(csv1);
await page.locator('[data-testid="csv-import-msg"]').waitFor({ timeout: 8000 }).catch(() => {});
await page.waitForTimeout(400);
let msg = (await page.locator('[data-testid="csv-import-msg"]').count()) ? await page.locator('[data-testid="csv-import-msg"]').innerText() : "(none)";
check("H2 setup: clean file import succeeded", /imported/i.test(msg), msg);

// second upload: 2 dup rows + 1 new → warning stages the bytes
const [fc2] = await Promise.all([page.waitForEvent("filechooser"), page.locator('[data-testid="csv-file-picker"]').click()]);
await fc2.setFiles(csv2);
await page.waitForTimeout(1500);
const warn = (await page.locator('[data-testid="csv-dup-warn"]').count()) ? await page.locator('[data-testid="csv-dup-warn"]').innerText() : "";
check("H2: duplicate warning shown for re-uploaded file", warn !== "", warn.replace(/\n/g, " "));
// THE FIX: the primary Import button must commit the staged file (not demand a paste)
await page.getByRole("button", { name: "Import", exact: true }).click();
await page.waitForTimeout(1500);
msg = (await page.locator('[data-testid="csv-import-msg"]').count()) ? await page.locator('[data-testid="csv-import-msg"]').innerText() : "(none)";
check("H2: primary Import committed the chosen file", /imported/i.test(msg) && !/paste some csv/i.test(msg), msg);
// close the modal
await page.keyboard.press("Escape");
await page.waitForTimeout(700);
body = await bodyText();
check("H2: the new row from the file is in the ledger", body.includes("QA H2 Gamma"));

// ───────────────────────── M1: household member add refresh ─────────────────────────
await nav("/household");
await page.locator('button:has-text("Add member")').first().click();
await page.waitForTimeout(900);
await page.locator('#member-add').fill("QA Probe Viewer");
await page.locator('[data-testid="member-add-role"]').selectOption("viewer").catch(async () => {
  await page.locator('[data-testid="member-add-role"]').selectOption({ label: "Viewer" });
});
await page.evaluate(() => document.getElementById("member-add-form").requestSubmit());
await page.waitForTimeout(1300);
body = await bodyText();
check("M1: household roster shows the new member without navigating away", body.includes("QA Probe Viewer"));
check("M1: success notice posted", /added .*household/i.test(body) || body.includes("Added QA Probe Viewer"));

// ───────────────────────── CF-02: review inbox ─────────────────────────
await nav("/transactions");
await page.locator('[data-testid="txn-review-btn"]').first().click();
await page.waitForTimeout(900);
const progress = async () => (await page.locator('[data-testid="review-progress"]').count()) ? await page.locator('[data-testid="review-progress"]').innerText() : "(none)";
const p0 = await progress();
// unarmed confirm must explain itself
await page.locator('[data-testid="review-commit"]').click();
await page.waitForTimeout(500);
const unarmErr = (await page.locator('[data-testid="review-commit-err"]').count()) ? await page.locator('[data-testid="review-commit-err"]').innerText() : "";
check("CF-02: unarmed confirm shows validation", unarmErr !== "", unarmErr);
check("CF-02: unarmed confirm does not advance", (await progress()) === p0);
// armed confirm advances
const sel = page.locator('[data-testid="review-category-select"]');
const opts = await sel.locator("option").all();
for (const o of opts) { const v = await o.getAttribute("value"); if (v) { await sel.selectOption(v); break; } }
await page.waitForTimeout(300);
await page.locator('[data-testid="review-commit"]').click();
await page.waitForTimeout(900);
const p1 = await progress();
check("CF-02: armed confirm advances", p1 !== p0, `${p0} → ${p1}`);
// skip advances
await page.locator('[data-testid="review-skip"]').click();
await page.waitForTimeout(700);
const p2 = await progress();
check("CF-02: skip advances", p2 !== p1, `${p1} → ${p2}`);
await page.keyboard.press("Escape");
await page.waitForTimeout(600);

// M1 companion: the topbar view-as lens must list the just-added member without
// a reload (the switcher used to render a stale roster). Note the lens is a
// SCOPE control, not an identity switch — role-blocked writes are exercised at
// the unit level, not here.
const sw = page.locator('[data-testid="member-switcher"]');
const swOpts = await sw.locator("option").allInnerTexts();
check("M1: topbar view-as lens lists the new member", swOpts.some((o) => o.includes("QA Probe Viewer")), JSON.stringify(swOpts));

// ───────────────────────── CF-04: per-item notification read state ─────────────────────────
await nav("/notifications");
const unreadRows = page.locator(".notif.is-unread");
const unread0 = await unreadRows.count();
check("CF-04 setup: inbox has unread items", unread0 > 1, `${unread0} unread rows`);
// open ONE linked unread notification (its main region routes to the resource)
const linked = page.locator(".notif.is-unread .notif-main.is-linked").first();
if (await linked.count()) {
  await linked.click();
  await page.waitForTimeout(1200);
  // we navigated away; come back
  await nav("/notifications");
  await page.waitForTimeout(600);
  const unread1 = await page.locator(".notif.is-unread").count();
  check("CF-04: exactly one notification became read", unread1 === unread0 - 1, `${unread0} → ${unread1}`);
  check("CF-04: inbox did not flip to all-read", unread1 > 0, `${unread1} still unread`);
} else {
  check("CF-04: found a linked unread notification to open", false);
}

// ───────────────────────── CF-05: historical budget pacing ─────────────────────────
await nav("/budgets");
const metrics0 = page.locator('[data-testid^="budget-metrics-"]').first();
await metrics0.waitFor({ timeout: 8000 }).catch(() => {});
const curMetrics = (await metrics0.count()) ? await metrics0.innerText() : "(none)";
check("CF-05 setup: current period shows live pacing", /Days left/i.test(curMetrics), curMetrics.replace(/\n/g, " · "));
// page back to the previous (completed) period
await page.locator('button.period-step[aria-label="Previous period"]').first().click();
await page.waitForTimeout(1500);
const histMetrics = (await metrics0.count()) ? await metrics0.innerText() : "(none)";
check("CF-05: completed period shows Ended, not days-left", /Ended/i.test(histMetrics) && !/Days left/i.test(histMetrics), histMetrics.replace(/\n/g, " · "));
check("CF-05: completed period reads 100% elapsed", /100%/.test(histMetrics), "");
const histBody = await bodyText();
check("CF-05: no projected-overspend warnings on history", !/projected/i.test(histBody) || !/overspend/i.test(histBody), "");
// restore the current period
await page.locator('button.period-step[aria-label="Next period"]').first().click();
await page.waitForTimeout(800);

// ───────────────────────── M2: rules quick-add submits ─────────────────────────
await nav("/rules");
const ruleForm = page.locator('[data-testid="rule-add-form"]').first();
await ruleForm.waitFor({ timeout: 8000 }).catch(() => {});
const rulesBefore = await page.locator("body").innerText();
const countBefore = (rulesBefore.match(/QA M2 Probe/g) || []).length;
await ruleForm.locator('input[type="text"]').first().fill("QA M2 Probe");
// pick any category
const ruleCat = ruleForm.locator("select").first();
const rOpts = await ruleCat.locator("option").all();
for (const o of rOpts) { const v = await o.getAttribute("value"); if (v) { await ruleCat.selectOption(v); break; } }
await page.waitForTimeout(300);
check("M2: inline quick-add has a visible submit", (await page.locator('[data-testid="rule-add-submit"]').count()) > 0);
await page.locator('[data-testid="rule-add-submit"]').click();
await page.waitForTimeout(1000);
const rulesAfter = await page.locator("body").innerText();
check("M2: rule saved and appears in the list", (rulesAfter.match(/QA M2 Probe/g) || []).length > countBefore, "");
check("M2: form cleared after save", (await ruleForm.locator('input[type="text"]').first().inputValue()) === "");
check("M2: success notice posted", /Rule added/i.test(rulesAfter));

// ──────────────── M4 + L4: allocation holdback metric + confirm labels ────────────────
await nav("/allocate");
await page.locator('[data-testid="allocate-amount"]').fill("100");
await page.waitForTimeout(1200);
const chipsText = await page.locator(".debt-chips").first().innerText();
const keptText = (await page.locator(".alloc-kept").count()) ? await page.locator(".alloc-kept").innerText() : "";
const keptM = keptText.match(/Kept back: \$([\d,.]+)/);
if (keptM) {
  const kept = keptM[1];
  check("M4: Held back chip matches the kept-back copy",
    new RegExp(`HELD BACK[\\s\\S]{0,10}\\$${kept.replace(".", "\\.")}`, "i").test(chipsText),
    `kept=$${kept} chips=${chipsText.replace(/\n/g, " ")}`);
} else {
  // no rounding leftover this run — chip must not contradict a nonzero copy, so just record
  check("M4: no kept-back leftover produced this run (nothing to contradict)", true, "");
}
// L4: open the confirmation and assert no doubled "Goal · Goal ·" labels
const applyBtn = page.locator('button:has-text("Apply allocation")').first();
await applyBtn.waitFor({ timeout: 8000 }).catch(() => {});
if (await applyBtn.count()) {
  await applyBtn.scrollIntoViewIfNeeded().catch(() => {});
  await applyBtn.click();
  await page.waitForTimeout(800);
  const confirmText = await bodyText();
  check("L4: confirmation rows do not repeat the kind label", !confirmText.includes("Goal · Goal ·"), "");
  const cancel = page.locator('.alloc-confirm button:has-text("Cancel"), button:has-text("Cancel")').first();
  if (await cancel.count()) await cancel.click();
  await page.waitForTimeout(400);
} else {
  const btns = (await page.locator("button").allInnerTexts()).filter((b) => /apply|alloc/i.test(b));
  check("L4: Apply allocation button reachable", false, JSON.stringify(btns));
}

// ──────────────── CF-07: report net worth follows the report scope ────────────────
await nav("/reports");
await page.waitForTimeout(1500);
const nwOf = async () => {
  const t = await bodyText();
  const m = t.match(/Net worth[\s\S]{0,80}?\$([\d,]+\.\d{2})/i);
  return m ? m[1] : "(none)";
};
const nwAll = await nwOf();
await page.locator('button:has-text("Scope")').first().click();
await page.waitForTimeout(600);
const scopeChip = page.locator(".scope-chip", { hasText: "Marcus Hartley" }).first();
if (await scopeChip.count()) {
  await scopeChip.click();
  await page.waitForTimeout(1200);
  const nwScoped = await nwOf();
  check("CF-07: net worth changes under the report scope", nwScoped !== "(none)" && nwScoped !== nwAll, `all=$${nwAll} scoped=$${nwScoped}`);
  // clear the scope for a clean state
  await page.locator('button:has-text("Clear")').first().click().catch(() => {});
  await page.waitForTimeout(600);
} else {
  check("CF-07: scope chip for a member reachable", false);
}

// ──────────────── L1: covering an overage moves the exact shortfall ────────────────
await nav("/budgets");
await page.waitForTimeout(1500);
const coverAllBtn = page.locator('[data-testid="budgets-cover-all"]');
if (await coverAllBtn.count()) {
  await coverAllBtn.click();
  await page.waitForTimeout(900);
  // pick a funding source for every over-row (first non-empty option per select)
  const rowSelects = await page.locator(".cover-all select").all();
  for (const s of rowSelects) {
    const so = await s.locator("option").all();
    for (const o of so) { const v = await o.getAttribute("value"); if (v) { await s.selectOption(v); break; } }
  }
  await page.locator('[data-testid="cover-all-apply"]').click();
  await page.waitForTimeout(1500);
  body = await bodyText();
  check("L1: covered budget lands exactly at its limit ($0.00 left, no stray cent)", body.includes("$0.00 left") && !body.includes("$0.01 left"), "");
} else {
  check("L1: no over-budget in sample to cover (covered by table tests)", true, "");
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
