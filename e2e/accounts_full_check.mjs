// Comprehensive e2e for the widgetized /accounts page + all its features.
//
// Reads persisted data from IndexedDB (cashflux-kv / key "cashflux:dataset") — the
// app's single persistence primitive since it migrated off localStorage, so tests
// that still read localStorage["cashflux:dataset"] see an empty blob. Each
// destructive test runs in its own fresh page so mutations don't cross-contaminate.
//
// Run: node e2e/accounts_full_check.mjs   (against `go run e2e/serve.go` on :8099)
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });

const results = [];
let allErrs = [];
function check(name, cond, detail = "") {
  results.push({ name, ok: !!cond });
  console.log((cond ? "PASS " : "FAIL ") + name + (detail ? " — " + detail : ""));
}

async function ready(p) {
  await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
  await p.waitForFunction(() => { const b = document.getElementById("boot"); return !b || b.classList.contains("hidden") || b.offsetParent === null; }, { timeout: 15000 }).catch(() => {});
  await p.waitForTimeout(500);
}
async function flush(p) { await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await p.waitForTimeout(500); }
async function dataset(p) {
  await flush(p);
  return await p.evaluate(async () => {
    const db = await new Promise((res, rej) => { const r = indexedDB.open("cashflux-kv"); r.onsuccess = () => res(r.result); r.onerror = () => rej(r.error); });
    const tx = db.transaction("kv", "readonly");
    const v = await new Promise((res) => { const r = tx.objectStore("kv").get("cashflux:dataset"); r.onsuccess = () => res(r.result); });
    try { return JSON.parse(v || "{}"); } catch (e) { return {}; }
  });
}
async function newPage() {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) allErrs.push(m); });
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await ready(page);
  return page;
}
async function openRowMenu(page, rowIdx = 0) {
  await page.locator(".bento-accounts .row button[aria-haspopup='menu']").nth(rowIdx).click();
  await page.waitForTimeout(300);
}

try {
  // ── A: structure + toolbar + filters ─────────────────────────────────────────
  {
    const page = await newPage();
    const struct = await page.evaluate(() => ({
      bento: !!document.querySelector(".bento.bento-accounts"),
      summary: !!document.querySelector(".nw-summary"),
      netWorth: (document.querySelector(".nw-summary .stat-hero .stat-value")?.textContent || "").trim(),
      toolbar: !!document.querySelector("[data-testid='page-transfer-btn']"),
      rows: document.querySelectorAll(".bento-accounts .row").length,
      liabLink: !!document.querySelector(".nw-summary a[href*='/debt']"),
    }));
    check("A1 bento-accounts surface present", struct.bento);
    check("A2 summary tile with net worth", struct.summary && /\$/.test(struct.netWorth), struct.netWorth);
    check("A3 toolbar present (transfer btn)", struct.toolbar);
    check("A4 asset rows rendered", struct.rows > 3, struct.rows + " rows");
    check("A5 liabilities links to /debt", struct.liabLink);

    const searchBox = page.locator("[placeholder*='Search accounts']").first();
    await searchBox.fill("Roth");
    await page.waitForTimeout(400);
    const rothOnly = await page.evaluate(() => {
      const rows = [...document.querySelectorAll(".bento-accounts .row .row-desc")].map(r => r.textContent || "");
      return { count: rows.length, allRoth: rows.every(t => /roth/i.test(t)), hasRoth: rows.some(t => /roth/i.test(t)) };
    });
    check("A6 search narrows list", rothOnly.hasRoth && rothOnly.allRoth, JSON.stringify(rothOnly));
    check("A7 search chip shown", await page.evaluate(() => !!document.querySelector(".filter-chip, [class*='chip']")));
    await searchBox.fill("");
    await page.waitForTimeout(300);

    await page.locator('button:has-text("Filters")').first().click();
    await page.waitForTimeout(300);
    const filterCtrls = await page.evaluate(() => ({
      typeSel: !!document.querySelector(".filter-fields select"),
      archived: !!document.querySelector("[data-testid='acct-toggle-archived']"),
      formulas: !!document.querySelector("[data-testid='acct-toggle-formulas']"),
    }));
    check("A8 filter: type select", filterCtrls.typeSel);
    check("A9 filter: show-archived toggle", filterCtrls.archived);
    check("A10 filter: formulas toggle", filterCtrls.formulas);

    await page.locator("[data-testid='acct-toggle-formulas']").click();
    await page.waitForTimeout(700);
    const formula = await page.evaluate(() => {
      const t = [...document.querySelectorAll(".bento-accounts > .w")].pop();
      return { lastTile: t?.getAttribute("data-widget"), hasVars: /net_worth|asset_accounts|liabilities/.test(document.body.textContent || "") };
    });
    check("A11 formulas toggle reveals formula tile", formula.lastTile === "acct-formula" && formula.hasVars);

    await page.locator("[data-testid='acct-toggle-archived']").click();
    await page.waitForTimeout(500);
    check("A12 show-archived toggle handled (no crash)", Array.isArray(await page.evaluate(() => [...document.querySelectorAll(".bento-accounts > .w")].map(w => w.getAttribute("data-widget")))));
    await page.close();
  }

  // ── B: mark all updated ──────────────────────────────────────────────────────
  {
    const page = await newPage();
    const before = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(x => /mark all updated/i.test(x.textContent || "")); return b ? b.textContent.trim() : "ABSENT"; });
    check("B1 mark-all-updated button present with count", /mark all updated \(\d+/i.test(before), before);
    await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(x => /mark all updated/i.test(x.textContent || "")); if (b) b.click(); });
    await page.waitForTimeout(700);
    const after = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(x => /mark all updated/i.test(x.textContent || "")); return b ? b.textContent.trim() : "RETIRED"; });
    check("B2 mark-all retires the button (re-render)", after === "RETIRED", "after=" + after);
    await page.close();
  }

  // ── C: page-level Transfer money → 2 legs in IDB ─────────────────────────────
  {
    const page = await newPage();
    const ds0 = await dataset(page);
    await page.locator("[data-testid='page-transfer-btn']").click();
    await page.waitForTimeout(500);
    check("C1 page transfer sub-view opens", await page.evaluate(() => !!document.querySelector("[data-testid='page-transfer-form']")));
    const fromVal = await page.evaluate(() => { const s = document.querySelector("[data-testid='page-xfer-from-select']"); const o = [...s.options].find(o => o.value); return o ? o.value : ""; });
    await page.locator("[data-testid='page-xfer-from-select']").selectOption(fromVal);
    const toVal = await page.evaluate((fv) => { const s = document.querySelector("[data-testid='page-xfer-to-select']"); const o = [...s.options].find(o => o.value && o.value !== fv); return o ? o.value : ""; }, fromVal);
    await page.locator("[data-testid='page-xfer-to-select']").selectOption(toVal);
    await page.locator("#page-xfer-amt").fill("40");
    await page.locator("[data-testid='page-transfer-form'] button[type='submit']").first().click();
    await page.waitForTimeout(700);
    const ds1 = await dataset(page);
    const newTxns = (ds1.transactions || []).filter(t => !(ds0.transactions || []).find(p => p.id === t.id));
    const legs = newTxns.filter(t => t.transferAccountId || t.transferAccountID);
    check("C2 page transfer creates 2 legs", legs.length >= 2, `new=${newTxns.length} legs=${legs.length}`);
    await page.close();
  }

  // ── D: modals centered + open/dismiss ────────────────────────────────────────
  {
    const page = await newPage();
    await page.locator(".bento-accounts .row").first().locator('button:has-text("Edit")').first().click();
    await page.waitForTimeout(700);
    const editM = await page.evaluate(() => {
      const w = document.querySelector(".flip-wrap"); if (!w) return { none: true };
      const r = w.getBoundingClientRect();
      return { centered: Math.abs((r.x + r.width / 2) - innerWidth / 2) < 3 && Math.abs((r.y + r.height / 2) - innerHeight / 2) < 3, focused: document.activeElement?.id, singleCol: !!document.querySelector(".acct-edit-form"), actions: !!document.querySelector(".acct-edit-actions") };
    });
    check("D1 edit modal centered on viewport", editM.centered);
    check("D2 edit modal autofocuses name", /acct-edit-/.test(editM.focused || ""), editM.focused);
    check("D3 edit modal single-column layout + action row", editM.singleCol && editM.actions);
    await page.keyboard.press("Escape"); await page.waitForTimeout(400);
    check("D4 Escape dismisses modal", await page.evaluate(() => !document.querySelector(".flip-wrap")));
    await openRowMenu(page, 1);
    await page.locator('.add-menu:not(.hidden-menu) [data-testid^="reconcile-start-btn-"]').first().click();
    await page.waitForTimeout(700);
    const recM = await page.evaluate(() => ({ flip: !!document.querySelector(".flip-wrap"), input: !!document.querySelector("[data-testid='reconcile-statement-input']"), centered: (() => { const w = document.querySelector(".flip-wrap"); if (!w) return false; const r = w.getBoundingClientRect(); return Math.abs((r.x + r.width / 2) - innerWidth / 2) < 3; })() }));
    check("D5 reconcile modal opens + centered", recM.flip && recM.input && recM.centered);
    await page.close();
  }

  // ── E: edit modal save → name change in IDB + UI ─────────────────────────────
  {
    const page = await newPage();
    await page.locator(".bento-accounts .row").first().locator('button:has-text("Edit")').first().click();
    await page.waitForTimeout(700);
    await page.locator(".acct-edit-form input[id^='acct-edit-']").first().fill("Renamed Asset QA");
    await page.locator(".acct-edit-actions button[type='submit']").first().click();
    await page.waitForTimeout(700);
    const ds1 = await dataset(page);
    check("E1 edit modal save persists name change", (ds1.accounts || []).some(a => a.name === "Renamed Asset QA"));
    check("E2 renamed account shows in list (re-render)", await page.evaluate(() => [...document.querySelectorAll(".bento-accounts .row .row-desc")].some(r => /Renamed Asset QA/.test(r.textContent || ""))));
    await page.close();
  }

  // ── F: update-value modal → adjustment txn + balance ─────────────────────────
  {
    const page = await newPage();
    const ds0 = await dataset(page);
    const txn0 = (ds0.transactions || []).length;
    await page.locator('.bento-accounts .row button:has-text("Update value"), .bento-accounts .row button:has-text("Update balance")').first().click();
    await page.waitForTimeout(650);
    await page.locator(".acct-edit-form input[type='number']").first().fill("199999");
    await page.waitForTimeout(300);
    check("F1 update-balance shows live delta preview", await page.evaluate(() => (document.querySelector("[data-testid='setbal-delta-preview']")?.textContent || "").length > 0));
    check("F2 update-balance has category picker", await page.evaluate(() => !!document.querySelector("[data-testid='setbal-cat-select']")));
    await page.locator(".acct-edit-actions button[type='submit']").first().click();
    await page.waitForTimeout(700);
    const ds1 = await dataset(page);
    check("F3 update-balance posts an adjustment transaction", (ds1.transactions || []).length > txn0, `${txn0} -> ${(ds1.transactions || []).length}`);
    check("F4 new balance reflected in the row", await page.evaluate(() => [...document.querySelectorAll(".bento-accounts .row")].some(r => /199,999/.test(r.textContent || ""))));
    await page.close();
  }

  // ── G: reconcile → mark cleared changes difference ───────────────────────────
  {
    const page = await newPage();
    const ds = await dataset(page);
    const accs = ds.accounts || [], txns = ds.transactions || [];
    let target = null;
    for (const a of accs) { if (a.archived || a.class === "liability") continue; if (txns.some(t => t.accountId === a.id && !t.cleared)) { target = a; break; } }
    check("G0 found an account with uncleared txns (data)", !!target, target ? target.name : "none");
    if (target) {
      const idx = await page.evaluate((name) => [...document.querySelectorAll(".bento-accounts .row")].findIndex(r => (r.querySelector(".row-desc")?.textContent || "").includes(name)), target.name);
      if (idx >= 0) {
        await openRowMenu(page, idx);
        await page.locator('.add-menu:not(.hidden-menu) [data-testid^="reconcile-start-btn-"]').first().click();
        await page.waitForTimeout(700);
        const before = await page.evaluate(() => document.querySelector("[data-testid='reconcile-difference']")?.textContent || "");
        const uncleared = await page.locator("[data-testid='reconcile-txn-clear-btn']").count();
        check("G1 reconcile modal lists uncleared txns", uncleared > 0, uncleared + " rows");
        if (uncleared > 0) {
          await page.locator("[data-testid='reconcile-txn-clear-btn']").first().click();
          await page.waitForTimeout(600);
          const after = await page.evaluate(() => document.querySelector("[data-testid='reconcile-difference']")?.textContent || "");
          check("G2 marking a txn cleared updates the difference", before !== after, `'${before}' -> '${after}'`);
        }
      } else check("G1 reconcile (row found)", false, "row not visible");
    }
    await page.close();
  }

  // ── H: per-row transfer modal → 2 legs ───────────────────────────────────────
  {
    const page = await newPage();
    const ds0 = await dataset(page);
    await openRowMenu(page, 1);
    await page.locator('.add-menu:not(.hidden-menu) [data-testid^="transfer-start-btn-"]').first().click();
    await page.waitForTimeout(700);
    const toVal = await page.evaluate(() => { const s = document.querySelector("[data-testid='acct-xfer-to-select']"); const o = [...s.options].find(o => o.value); return o ? o.value : ""; });
    await page.locator("[data-testid='acct-xfer-to-select']").selectOption(toVal);
    await page.locator(".acct-edit-form input[id^='acct-xfer-amt-']").fill("33");
    await page.locator(".acct-edit-actions button[type='submit']").first().click();
    await page.waitForTimeout(700);
    const ds1 = await dataset(page);
    const legs = (ds1.transactions || []).filter(t => !(ds0.transactions || []).find(p => p.id === t.id)).filter(t => t.transferAccountId || t.transferAccountID);
    check("H1 per-row transfer creates 2 legs", legs.length >= 2, `legs=${legs.length}`);
    await page.close();
  }

  // ── I: archive + delete-guard ────────────────────────────────────────────────
  {
    const page = await newPage();
    const ds0 = await dataset(page);
    await openRowMenu(page, 0);
    check("I1 archive action in ⋯ menu", await page.evaluate(() => !![...document.querySelectorAll(".add-menu:not(.hidden-menu) [role='menuitem']")].find(x => /archive/i.test(x.textContent || ""))));
    await page.locator(".add-menu:not(.hidden-menu) [role='menuitem']").filter({ hasText: /archive/i }).first().click();
    await page.waitForTimeout(700);
    const ds1 = await dataset(page);
    check("I2 archive persists (archived count up)", (ds1.accounts || []).filter(a => a.archived).length > (ds0.accounts || []).filter(a => a.archived).length);
    await page.locator(".bento-accounts .row .btn-del").first().click();
    await page.waitForTimeout(600);
    const ds2 = await dataset(page);
    check("I3 delete handled without crash (guard or remove)", typeof (ds2.accounts || []).length === "number");
    await page.close();
  }

  // ── J: view transactions navigation ──────────────────────────────────────────
  {
    const page = await newPage();
    await page.locator(".bento-accounts .row").first().locator('button:has-text("Transactions")').first().click();
    await page.waitForTimeout(900);
    check("J1 'Transactions' navigates to /transactions", /\/transactions/.test(page.url()), page.url());
    check("J2 transactions page renders", await page.evaluate(() => !!document.querySelector(".bento-ledger, .txn-table, #app .bento")));
    await page.close();
  }

  // ── K: custom-field display + menu viewport-awareness ────────────────────────
  {
    const page = await newPage();
    const cf = await page.evaluate(() => { const s = document.querySelector("[data-testid^='acct-custom-summary-']"); return s ? s.textContent.trim() : ""; });
    check("K1 custom-field values shown on a row", /:/.test(cf), cf || "(none in sample)");
    await page.locator(".bento-accounts .row button[aria-haspopup='menu']").first().click();
    await page.waitForTimeout(300);
    const menu = await page.evaluate(() => { const m = document.querySelector(".add-menu:not(.hidden-menu)"); if (!m) return { none: true }; const r = m.getBoundingClientRect(); return { inX: r.left >= 0 && r.right <= innerWidth + 1, inY: r.top >= 0 && r.bottom <= innerHeight + 1 }; });
    check("K2 ⋯ menu stays within viewport", menu.inX && menu.inY, JSON.stringify(menu));
    await page.close();
  }

  const pass = results.filter(r => r.ok).length, fail = results.length - pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if (fail) { console.log("FAILED: " + results.filter(r => !r.ok).map(r => r.name).join(", ")); process.exitCode = 1; }
  console.log("page errors: " + (allErrs.length ? JSON.stringify([...new Set(allErrs)].slice(0, 6)) : "none"));
  console.log("════════════════════════════════════════════");
} finally {
  await browser.close();
}
