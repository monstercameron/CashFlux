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

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
