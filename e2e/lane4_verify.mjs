// lane4_verify.mjs — e2e verification for the lane-4 remediation batch:
//   #48 reconciliation discrepancy-resolution toolkit (adjust / investigate /
//       force-complete / save-draft-resume / reopen-last)
//   (later sections appended per task: #77 #55 #57 #63 #58 #53)
// Usage: node e2e/lane4_verify.mjs [port]   (default 8114, lane-4's server)
import { chromium } from "playwright";

const PORT = process.argv[2] || "8114";
const BASE = `http://127.0.0.1:${PORT}`;
const SHOTS = process.env.LANE4_SHOTS || "";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};
const shot = async (page, name) => { if (SHOTS) await page.screenshot({ path: `${SHOTS}/${name}.png`, fullPage: false }); };

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1400); };
const bodyText = async () => await page.locator("body").innerText();

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

// ─────────────── #48: reconciliation discrepancy resolution ───────────────
await nav("/accounts");
await page.waitForTimeout(1200);

const startBtn = page.locator('[data-testid^="reconcile-start-btn-"]').first();
check("#48: a reconcile-eligible account row exists", (await startBtn.count()) > 0);
const acctID = (await startBtn.getAttribute("data-testid")).replace("reconcile-start-btn-", "");
const row = page.locator(`[data-testid="acct-row-${acctID}"]`);

const openRecon = async () => {
  await row.locator('button[aria-haspopup="menu"]').first().click();
  await page.waitForTimeout(400);
  await page.locator(`[data-testid="reconcile-start-btn-${acctID}"]`).click();
  await page.waitForTimeout(900); // FlipPanel flip
};
const modal = page.locator('[data-testid="reconcile-statement-mode"]');
const stmtInput = page.locator('[data-testid="reconcile-statement-input"]');

// 1) Mismatched statement → resolution row appears (the CF-02 dead-end is gone).
await openRecon();
check("#48: reconcile modal open", await modal.isVisible());
await stmtInput.fill("999999.99");
await page.waitForTimeout(600);
check("#48: resolve row offered on a difference", await page.locator('[data-testid="reconcile-resolve"]').isVisible());
check("#48: adjust action offered", await page.locator('[data-testid="reconcile-adjust"]').isVisible());
check("#48: investigate action offered", await page.locator('[data-testid="reconcile-investigate"]').isVisible());
check("#48: force-complete action offered", await page.locator('[data-testid="reconcile-force"]').isVisible());
check("#48: save-draft offered in the footer", await page.locator('[data-testid="reconcile-save-draft"]').isVisible());
check("#48: record button withheld while unbalanced", (await page.locator('[data-testid="reconcile-done"]').count()) === 0);
await shot(page, "48-resolve-row");

// 2) Save & finish later → closes; reopening resumes the draft.
await page.locator('[data-testid="reconcile-save-draft"]').click();
await page.waitForTimeout(900);
check("#48: save-draft closes the modal", !(await modal.isVisible().catch(() => false)));
check("#48: save-draft toast", /pick up where you left off/i.test(await bodyText()));
await openRecon();
check("#48: draft re-seeds the statement balance", (await stmtInput.inputValue()) === "999999.99", await stmtInput.inputValue());
check("#48: resumed-draft note shown", await page.locator('[data-testid="reconcile-draft-note"]').isVisible());

// 3) Investigate → jumps to this account's cleared ledger (draft is kept).
await page.locator('[data-testid="reconcile-investigate"]').click();
await page.waitForTimeout(1400);
check("#48: investigate navigates to /transactions", page.url().includes("/transactions"), page.url());
await nav("/accounts");

// 4) Post adjustment → difference hits zero → record → history entry.
await openRecon();
check("#48: draft survived the investigate detour", (await stmtInput.inputValue()) === "999999.99");
const histBefore = await page.locator('[data-testid="reconcile-history-row"]').count();
await page.locator('[data-testid="reconcile-adjust"]').click();
await page.waitForTimeout(900);
check("#48: adjustment posts and balances the difference", await page.locator('[data-testid="reconcile-confirmed"]').isVisible());
check("#48: adjustment toast (undoable)", /adjustment to match the statement/i.test(await bodyText()));
await shot(page, "48-adjusted-balanced");
await page.locator('[data-testid="reconcile-done"]').click();
await page.waitForTimeout(900);
check("#48: record closes the modal", !(await modal.isVisible().catch(() => false)));

// 5) Reopen last → the recorded event pops back out and re-seeds the form.
await openRecon();
const histAfter = await page.locator('[data-testid="reconcile-history-row"]').count();
check("#48: recording added a history row", histAfter === histBefore + 1, `${histBefore} → ${histAfter}`);
check("#48: reopen-last affordance by the history", await page.locator('[data-testid="reconcile-reopen"]').isVisible());
await page.locator('[data-testid="reconcile-reopen"]').click();
await page.waitForTimeout(700);
check("#48: reopen removes the newest history row", (await page.locator('[data-testid="reconcile-history-row"]').count()) === histAfter - 1);
check("#48: reopen re-seeds the statement figure", (await stmtInput.inputValue()).replace(/,/g, "") === "999999.99", await stmtInput.inputValue());
check("#48: reopen toast", /reopened the .* reconciliation/i.test(await bodyText()));

// 6) Force-complete with a fresh difference → confirmed via dialog → history
//    row carries the forced "off by" marker.
await stmtInput.fill("888888.88");
await page.waitForTimeout(600);
await page.locator('[data-testid="reconcile-force"]').click();
await page.waitForTimeout(600);
check("#48: force asks for explicit confirmation", await page.locator("#cf-dialog-confirm").isVisible());
await page.locator("#cf-dialog-confirm").click();
await page.waitForTimeout(1100);
check("#48: force closes the modal", !(await modal.isVisible().catch(() => false)));
check("#48: force toast names the unresolved difference", /unresolved difference/i.test(await bodyText()));
await openRecon();
check("#48: forced history row carries the off-by marker", (await page.locator('[data-testid="reconcile-history-forced"]').count()) > 0);
await shot(page, "48-forced-history");
// tidy: reopen (undo) the forced event so reruns start clean-ish
await page.locator('[data-testid="reconcile-reopen"]').click();
await page.waitForTimeout(500);
await page.keyboard.press("Escape");
await page.waitForTimeout(600);

// ─────────────── #77: Mark all updated — preview + undo ───────────────
await nav("/accounts");
await page.waitForTimeout(1200);
const markAllBtn = page.locator('[data-testid="acct-markall-btn"]');
check("#77: mark-all button present (stale accounts exist)", (await markAllBtn.count()) > 0);
if (await markAllBtn.count()) {
  // Preview: the confirm names the count and the accounts before writing.
  await markAllBtn.click();
  await page.waitForTimeout(500);
  const dlg = await bodyText();
  const previewM = dlg.match(/Mark (\d+) balances? as confirmed just now\? This updates: (.+?)\. You can undo/);
  check("#77: confirm previews the affected count + names", !!previewM, previewM ? `${previewM[1]}: ${previewM[2].slice(0, 60)}` : dlg.slice(0, 120));
  await shot(page, "77-markall-preview");
  // Cancel is a true no-op.
  await page.locator("#cf-dialog-cancel").click();
  await page.waitForTimeout(500);
  check("#77: cancel leaves the stale set untouched", (await markAllBtn.count()) > 0);
  // Confirm applies and the toast carries a live Undo.
  await markAllBtn.click();
  await page.waitForTimeout(500);
  await page.locator("#cf-dialog-confirm").click();
  await page.waitForTimeout(1200);
  check("#77: applied toast", /Marked .* as updated just now/i.test(await bodyText()));
  const undoBtn = page.locator(".toast-undo");
  check("#77: toast offers Undo", (await undoBtn.count()) > 0);
  check("#77: bulk action consumed the stale set", (await markAllBtn.count()) === 0);
  await shot(page, "77-markall-undo-toast");
  await undoBtn.click();
  // A whole-dataset restore + re-render can take a moment; poll up to ~6s.
  let undone = false;
  for (let i = 0; i < 12 && !undone; i++) {
    await page.waitForTimeout(500);
    undone = (await markAllBtn.count()) > 0;
  }
  check("#77: undo restores the stale balances (button returns)", undone);
}

// ─────────────── #55: pre-operation safety checkpoints ───────────────
await nav("/transactions");
await page.waitForTimeout(1500);
const ledgerRow = page.locator('tr[data-testid^="txn-row-"]').first();
await ledgerRow.waitFor({ timeout: 20000 });
const firstRowDesc = (await ledgerRow.locator(".row-desc-text").innerText()).trim();
const descCountBefore = await page.locator(".row-desc-text", { hasText: firstRowDesc }).count();
await ledgerRow.locator('input[type="checkbox"]').click();
await page.waitForTimeout(600);
check("#55: bulk bar appears on selection", (await page.locator('[data-testid="bulk-delete"]').count()) > 0);
await page.locator('[data-testid="bulk-delete"]').click();
await page.waitForTimeout(500);
await page.locator("#cf-dialog-confirm").click();
await page.waitForTimeout(1200);
check("#55: bulk delete applied", /deleted/i.test(await bodyText()));
// The checkpoint ring in Settings → Data now holds a pre-delete snapshot.
await nav("/settings");
await page.waitForTimeout(1200);
await page.locator('.set-tab-strip button', { hasText: "Data" }).first().click();
await page.waitForTimeout(900);
const ckptSection = page.locator('[data-testid="checkpoints-section"]');
check("#55: checkpoints section on the Data tab", await ckptSection.isVisible());
const ckptRow = page.locator('[data-testid="checkpoint-row"]').first();
check("#55: pre-delete checkpoint listed with a plain label",
  (await ckptRow.count()) > 0 && /Before deleting 1 transaction/.test(await ckptRow.innerText()),
  (await ckptRow.count()) ? (await ckptRow.innerText()).split("\n")[0] : "(none)");
await shot(page, "55-checkpoints-list");
// One-click restore brings the deleted transaction back.
await ckptRow.locator('[data-testid="checkpoint-restore"]').click();
await page.waitForTimeout(500);
check("#55: restore confirms first", await page.locator("#cf-dialog-confirm").isVisible());
await page.locator("#cf-dialog-confirm").click();
await page.waitForTimeout(2000);
check("#55: restore toast", /back to just before that operation/i.test(await bodyText()));
await nav("/transactions");
await page.waitForTimeout(1500);
await page.locator('tr[data-testid^="txn-row-"]').first().waitFor({ timeout: 20000 });
const descCountAfter = await page.locator(".row-desc-text", { hasText: firstRowDesc }).count();
check("#55: restored dataset has the deleted transaction back", descCountAfter === descCountBefore, `${descCountBefore} → ${descCountAfter} ("${firstRowDesc}")`);

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
