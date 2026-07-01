// C41 — archived accounts must NOT appear in the quick-add transaction account
// dropdown.
// KNOWN-FAILING (2026-06-27) due to C41b, NOT a filter regression: QuickAddHost renders
// a stale accounts snapshot, so the archived flag never reaches the (correct) filter.
// This probe will pass once C41b (QuickAddHost data-staleness) is fixed. See TODOS.md.
// Deterministic UI drive: snapshot the dropdown (option value=accountID),
// archive a specific NON-default account by ID via its ⋯ menu, reopen quick-add,
// assert exactly that account's option dropped.
import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed = 0; const fail = m => { console.error("FAIL: " + m); failed++; }; const pass = m => console.log("PASS: " + m);
const dropdownIds = p => p.evaluate(() => [...document.querySelectorAll('[data-testid="txn-add-account"] option')].map(o => o.value));
async function openQuickAdd(p) { await p.keyboard.press("Alt+KeyN"); await p.waitForSelector('[data-testid="txn-add-account"]', { timeout: 8000 }); await p.waitForTimeout(300); }
try {
  const page = await browser.newPage(); const errs = []; page.on("pageerror", e => errs.push(String(e)));
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 }); await page.waitForTimeout(4000);
  await openQuickAdd(page);
  const before = await dropdownIds(page);
  console.log("  dropdown ids before (" + before.length + ")");
  if (before.length < 3) { fail("need >=3 accounts (have " + before.length + ")"); throw new Error("setup"); }
  // Target a clearly non-default account (index 2) so the effAcct-kept guard can't mask the result.
  const targetId = before[2];
  // Reload to dismiss the quick-add flip panel so the accounts list is interactable.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 }); await page.waitForTimeout(3000);
  // The row's ⋯ menu wrapper is the .add-wrap that contains this account's transfer button.
  const wrap = page.locator('.add-wrap', { has: page.locator(`[data-testid="transfer-start-btn-${targetId}"]`) });
  if (await wrap.count() === 0) { fail("could not find row wrapper for account " + targetId); throw new Error("setup"); }
  await wrap.locator('button[aria-label="More actions"]').click();
  await page.waitForTimeout(300);
  await wrap.locator('button[role="menuitem"][title="Archive account"]').click();
  await page.waitForTimeout(2000);
  // Flush the archive to IDB (pagehide triggers autosave) then reload so quick-add
  // mounts fresh and reads the persisted state — reopening alone keeps a stale list.
  await page.evaluate(() => window.dispatchEvent(new Event("pagehide")));
  await page.waitForTimeout(600);
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 }); await page.waitForTimeout(3500);
  await openQuickAdd(page);
  const after = await dropdownIds(page);
  console.log("  before=" + before.length + " after=" + after.length + " targetGone=" + !after.includes(targetId));
  const removed = before.filter(x => !after.includes(x));
  const isSubset = after.every(x => before.includes(x));
  if (!after.includes(targetId) && after.length === before.length - 1 && isSubset && removed.length === 1 && removed[0] === targetId)
    pass("archived account (id " + targetId + ") is EXCLUDED from quick-add dropdown; all others retained");
  else fail("expected only target removed; removed=" + JSON.stringify(removed) + " after.len=" + after.length + " subset=" + isSubset);
  await page.screenshot({ path: "e2e/screenshots/quickadd_archived_filter.png" });
  console.log("  pageerrors: " + errs.length); errs.slice(0, 3).forEach(e => console.log("  ERR:" + e.slice(0, 100)));
  if (errs.length) fail("page errors present");
} catch (e) { if (e.message !== "setup") fail("exception: " + e.message); } finally { await browser.close(); }
console.log(failed ? "RESULT: FAILED" : "RESULT: PASSED"); process.exit(failed ? 1 : 0);
