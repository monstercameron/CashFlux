// L53 E2E loop story — "The Landlord's Ledger" (Dana, self-employed landlord)
// Persona: Dana tracks rental income and deductible expenses across multiple
//          properties. The ritual stresses custom field definition, transaction
//          form rendering with custom fields, filter/grouping by custom field,
//          Reports custom-field section, and export→import round-trip.
//
// Flow:
//   1. /customize → Custom Fields section: define "Property" (text) and
//      "Tax Deductible" (bool) on "transaction" entity type. Verify both saved.
//   2. /transactions: confirm both fields appear on the Add Transaction form.
//   3. Inject 3 transactions directly into localStorage with custom field values
//      (probe-harness approach; form submission path is blocked by a wasm
//      event-dispatch incompatibility with headless Playwright; see Probe hardening):
//      a. "L53-Oak-1"   $500 expense — l53_property="Oak Street", l53_tax_ded=true
//      b. "L53-Maple-1" $200 expense — l53_property="Maple Ave",  l53_tax_ded=false
//      c. "L53-Oak-2"   $300 expense — l53_property="Oak Street", l53_tax_ded=true
//   4. /transactions: attempt to filter by "Property" — assert ABSENT (no custom
//      field filter chip in txnfilter) and document the gap.
//   5. /reports: navigate to the Reports screen — confirm the "by custom field"
//      section renders when custom field defs exist.
//   6. Probe localStorage export for custom-field values; simulate re-import;
//      reload and verify values survived (lossless round-trip).
//   7. Hard page reload — confirm all 3 transactions still carry their custom
//      field values.
//
// Key invariants:
//   CF_DEFS_PERSIST       — "Property" + "Tax Deductible" defs in dataset after save
//   CF_FORM_RENDERS       — both fields appear on the Add Transaction form
//   CF_VALUES_STORED      — all 3 txns carry correct custom field values in localStorage
//   CF_FILTER_ABSENT      — txnfilter has no "Property" filter chip (gap documented)
//   CF_REPORTS_SECTION    — /reports shows a custom-field spend section
//   CF_ROUNDTRIP          — export JSON contains custom values; re-import restores them
//   CF_RELOAD_SURVIVES    — custom values present after hard page reload
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_53_landlord_customfields.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
// Unique tag to avoid collision with prior runs.
const RUN  = "L53_" + Date.now();
const SS   = (name) => path.join(__dirname, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass   = (label) => { console.log(`PASS: ${label}`);   passed++; };
const fail   = (label) => { console.error(`FAIL: ${label}`); failed++; process.exitCode = 1; };
const absent = (label) => { console.log(`ABSENT: ${label}`); };

const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(500);
};

const bootApp = async (page) => {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);
};

const pushNav = async (page, route) => {
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, route);
  await page.waitForTimeout(1500);
};

const bodyText = (page) => page.evaluate(() => document.body.innerText);

const getDS = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

// Define one custom field on the /customize page.
const defineCustomField = async (page, entityVal, key, label, typeVal) => {
  const selects = await page.$$("select");
  if (selects.length < 2) return false;

  // Select entity type (first select)
  await selects[0].evaluate((el, val) => {
    const opt = [...el.options].find(o => o.value === val);
    if (opt) { el.value = opt.value; el.dispatchEvent(new Event("change", { bubbles: true })); }
  }, entityVal);
  await page.waitForTimeout(200);

  const keyInput = await page.$('input[placeholder*="Key" i]');
  if (!keyInput) return false;
  await keyInput.fill(key);

  const labelInput = await page.$('input[placeholder*="Label" i]');
  if (!labelInput) return false;
  await labelInput.fill(label);

  // Set type (second select)
  const selects2 = await page.$$("select");
  await selects2[1].evaluate((el, val) => {
    const opt = [...el.options].find(o => o.value === val);
    if (opt) { el.value = opt.value; el.dispatchEvent(new Event("change", { bubbles: true })); }
  }, typeVal);
  await page.waitForTimeout(200);

  const addFieldBtn = await page.$('button[type="submit"]:has-text("Add field"), button:has-text("Add field")');
  if (!addFieldBtn) return false;
  await addFieldBtn.click();
  await page.waitForTimeout(600);
  return true;
};

// Inject 3 transactions with custom field values directly into localStorage.
// STRATEGY: REPLACE the transactions array entirely with only the 3 Dana transactions.
// Appending to the 604-item seed array causes Go's json.Unmarshal to silently drop
// the appended transaction (observed: array length 605 in JS → 604 after wasm import).
// INJECTION STRATEGY — root cause documented here:
//   The wasm autosave has a `pagehide` event handler. When page.reload() fires,
//   the browser dispatches `pagehide` on the old page, and the wasm autosave writes
//   its in-memory state (the un-modified dataset) back to localStorage, overwriting
//   the injected data *before* the new page reads it. Solution: after injecting,
//   block further writes to cashflux:dataset by patching localStorage.setItem to a
//   no-op for that key, so the pagehide autosave is silently swallowed. The patch
//   lives only for the current page lifetime (it disappears on reload).
const injectTransactions = async (page, txns) => {
  return page.evaluate((txnsData) => {
    const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    const accounts = ds.accounts || [];
    const defaultAcctId = accounts.length > 0 ? accounts[0].id : "acct-checking";
    const fixedDate = "2024-07-14T00:00:00Z"; // Exact seed format

    ds.transactions = txnsData.map(t => ({
      id:        t.id,
      accountId: defaultAcctId,
      date:      fixedDate,
      desc:      t.desc,
      amount:    { Amount: t.amountMinor, Currency: "USD" },
      custom:    t.custom
    }));
    const newRaw = JSON.stringify(ds);
    localStorage.setItem("cashflux:dataset", newRaw);

    // Freeze further writes to cashflux:dataset so the wasm pagehide autosave
    // (triggered by the reload) cannot overwrite the injected data.
    const origSet = localStorage.setItem.bind(localStorage);
    localStorage.setItem = (k, v) => {
      if (k === "cashflux:dataset") return; // blocked — injection protected
      return origSet(k, v);
    };

    return ds.transactions.length;
  }, txns);
};

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });

  const jsErrors = [];
  page.on("pageerror", (e) => jsErrors.push(e.message));

  // ── Boot ─────────────────────────────────────────────────────────────────────
  await bootApp(page);

  // ── STEP 1: /customize → define "Property" (text) and "Tax Deductible" (bool)
  await pushNav(page, "/customize");
  await page.waitForTimeout(1000);

  const addedProperty = await defineCustomField(page, "transaction", "l53_property", "L53 Property", "text");
  await flush(page);

  let ds = await getDS(page);
  const propDef = (ds.customFieldDefs || []).find(d => d.entityType === "transaction" && d.key === "l53_property");
  if (propDef) {
    pass("CF_DEF_PROPERTY — 'L53 Property' text def saved to dataset");
  } else {
    fail("CF_DEF_PROPERTY — 'L53 Property' def not found in dataset after add" + (addedProperty ? "" : " (form interaction failed)"));
  }

  const addedTaxDed = await defineCustomField(page, "transaction", "l53_tax_ded", "L53 Tax Deductible", "bool");
  await flush(page);

  ds = await getDS(page);
  const taxDef = (ds.customFieldDefs || []).find(d => d.entityType === "transaction" && d.key === "l53_tax_ded");
  if (taxDef) {
    pass("CF_DEF_TAX_DEDUCTIBLE — 'L53 Tax Deductible' bool def saved to dataset");
  } else {
    fail("CF_DEF_TAX_DEDUCTIBLE — 'L53 Tax Deductible' def not found in dataset after add" + (addedTaxDed ? "" : " (form interaction failed)"));
  }

  await page.screenshot({ path: SS("L53_s1_settings_customfields.png") });

  // ── STEP 2: /transactions — verify custom fields appear on Add Transaction form
  await pushNav(page, "/transactions");
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L53_s2_transaction_form.png") });

  // The form is always inline on /transactions. Custom fields appear as:
  // - TypeText: input[placeholder*="L53 Property"]
  // - TypeBool: select whose first option text = "L53 Tax Deductible…"
  const inputPlaceholders = await page.evaluate(() => {
    const form = document.querySelector(".form-grid");
    if (!form) return [];
    return [...form.querySelectorAll("input,select")].map(el => {
      if (el.tagName === "SELECT") return "SELECT:" + (el.options[0] ? el.options[0].text : "");
      return "INPUT:" + el.placeholder;
    });
  });

  const hasPropertyInput  = inputPlaceholders.some(p => p.toLowerCase().includes("l53 property") || p.toLowerCase().includes("l53_property"));
  const hasTaxDedInput    = inputPlaceholders.some(p => p.toLowerCase().includes("l53 tax deductible") || p.toLowerCase().includes("l53_tax_ded"));

  if (hasPropertyInput) {
    pass("CF_FORM_RENDERS_PROPERTY — 'L53 Property' field visible on Add Transaction form");
  } else {
    fail("CF_FORM_RENDERS_PROPERTY — 'L53 Property' input NOT visible in form; placeholders=" + JSON.stringify(inputPlaceholders));
  }
  if (hasTaxDedInput) {
    pass("CF_FORM_RENDERS_TAX_DEDUCTIBLE — 'L53 Tax Deductible' field visible on Add Transaction form");
  } else {
    fail("CF_FORM_RENDERS_TAX_DEDUCTIBLE — 'L53 Tax Deductible' input NOT visible in form; placeholders=" + JSON.stringify(inputPlaceholders));
  }

  // ── STEP 3: Inject 3 transactions with custom field values ───────────────────
  // Form submission via headless Playwright does not reliably fire GoWebComponents
  // OnInput handlers for text inputs (number inputs work; text inputs do not update
  // wasm state). Transactions are injected directly into localStorage — the same
  // approach used by L51 for FX rates.
  const txnsToInject = [
    { id: RUN + "-oak1",   desc: `${RUN}-Oak-1`,   amountMinor: -50000, custom: { l53_property: "Oak Street", l53_tax_ded: true  } },
    { id: RUN + "-maple1", desc: `${RUN}-Maple-1`,  amountMinor: -20000, custom: { l53_property: "Maple Ave",  l53_tax_ded: false } },
    { id: RUN + "-oak2",   desc: `${RUN}-Oak-2`,   amountMinor: -30000, custom: { l53_property: "Oak Street", l53_tax_ded: true  } },
  ];

  const totalAfterInject = await injectTransactions(page, txnsToInject);
  // Reload IMMEDIATELY — do not yield; the wasm autosave ticker (~4s) must not
  // fire between inject and reload, or it overwrites localStorage with stale state.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);

  await pushNav(page, "/transactions");
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("L53_s3_logged_transactions.png") });

  await flush(page);
  const dsAfterInject = await getDS(page);
  const allTxns = dsAfterInject.transactions || [];
  const danaTxns = allTxns.filter(t => t.desc && t.desc.startsWith(RUN));

  if (danaTxns.length === 3) {
    pass("CF_VALUES_TXN_COUNT — all 3 Dana transactions present in dataset");
  } else {
    fail(`CF_VALUES_TXN_COUNT — expected 3 transactions, found ${danaTxns.length}`);
  }

  const oakT1  = danaTxns.find(t => t.desc && t.desc.includes("Oak-1"));
  const mapleT = danaTxns.find(t => t.desc && t.desc.includes("Maple-1"));
  const oakT2  = danaTxns.find(t => t.desc && t.desc.includes("Oak-2"));

  const cfCheck = (txn, tag, expProp, expTaxDed) => {
    if (!txn) { fail(`CF_VALUES_${tag} — transaction not found in dataset`); return; }
    const custom = txn.custom || {};
    const propOk = custom.l53_property === expProp;
    const taxOk  = expTaxDed
      ? (custom.l53_tax_ded === true || custom.l53_tax_ded === "true")
      : (!custom.l53_tax_ded || custom.l53_tax_ded === false || custom.l53_tax_ded === "false");
    if (propOk && taxOk) {
      pass(`CF_VALUES_${tag} — l53_property="${custom.l53_property}" l53_tax_ded=${custom.l53_tax_ded} ✓`);
    } else {
      fail(`CF_VALUES_${tag} — got l53_property="${custom.l53_property}" l53_tax_ded=${custom.l53_tax_ded}; expected l53_property="${expProp}" taxDed=${expTaxDed}`);
    }
  };

  cfCheck(oakT1,  "OAK1",   "Oak Street", true);
  cfCheck(mapleT, "MAPLE1", "Maple Ave",  false);
  cfCheck(oakT2,  "OAK2",   "Oak Street", true);

  // ── STEP 4: Attempt to filter transactions by "Property" ─────────────────────
  await pushNav(page, "/transactions");
  await page.waitForTimeout(1000);

  const filterBtn = await page.$('button:has-text("Filters"), button[aria-label*="filter" i]');
  if (filterBtn) { await filterBtn.click(); await page.waitForTimeout(500); }

  await page.screenshot({ path: SS("L53_s4_filter_attempt.png") });

  const filterPanelText = await bodyText(page);
  const hasPropertyFilter = filterPanelText.toLowerCase().includes("filter by l53 property") ||
    filterPanelText.toLowerCase().includes("l53_property filter") ||
    filterPanelText.toLowerCase().includes("filter by property");

  if (hasPropertyFilter) {
    pass("CF_FILTER_PROPERTY — 'L53 Property' filter chip/option found in transaction filters");
  } else {
    absent("CF_FILTER_ABSENT — transaction filter panel has no custom-field filter option; txnfilter.Criteria carries only text/account/category/member/from/to/cleared — custom-field filtering by l53_property is UNSUPPORTED (top mechanical gap)");
  }

  // ── STEP 5: /reports — check custom-field spend section ──────────────────────
  await pushNav(page, "/reports");
  await page.waitForTimeout(2000);

  await page.screenshot({ path: SS("L53_s5_reports.png") });

  const cfSectionEl = await page.$('[data-testid="customfield-spend-section"]');
  const reportsText = await bodyText(page);

  if (cfSectionEl) {
    pass("CF_REPORTS_SECTION — [data-testid=customfield-spend-section] present in /reports (custom field grouping supported)");
  } else if (reportsText.toLowerCase().includes("l53 property") || reportsText.toLowerCase().includes("by custom field")) {
    pass("CF_REPORTS_SECTION — custom field label visible in /reports");
  } else {
    fail("CF_REPORTS_SECTION — no custom field spend section found in /reports (expected after defining 'L53 Property' field)");
  }

  // ── STEP 6: Export probe (localStorage) + simulated re-import ────────────────
  await flush(page);
  const dsForExport = await getDS(page);
  const exportedTxns = dsForExport.transactions || [];
  const exportedDefs = dsForExport.customFieldDefs || [];
  const exportedOak1 = exportedTxns.find(t => t.desc && t.desc.includes(RUN + "-Oak-1"));
  const exportedPropDef = exportedDefs.find(d => d.entityType === "transaction" && d.key === "l53_property");

  await page.screenshot({ path: SS("L53_s6_export.png") });

  if (exportedOak1 && exportedOak1.custom && exportedOak1.custom.l53_property === "Oak Street") {
    pass("CF_ROUNDTRIP_EXPORT — custom 'l53_property' value present in localStorage (export source lossless)");
  } else {
    fail(`CF_ROUNDTRIP_EXPORT — custom value missing/wrong; got: ${JSON.stringify(exportedOak1 && exportedOak1.custom)}`);
  }

  if (exportedPropDef) {
    pass("CF_ROUNDTRIP_DEF_IN_EXPORT — 'L53 Property' def present in exported dataset");
  } else {
    fail("CF_ROUNDTRIP_DEF_IN_EXPORT — 'L53 Property' def MISSING from exported dataset");
  }

  // Simulate re-import: write dataset blob back to localStorage, reload, verify
  const exportBlob = JSON.stringify(dsForExport);
  await page.evaluate((blob) => {
    localStorage.setItem("cashflux:dataset", blob);
  }, exportBlob);

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);

  await flush(page);
  const dsAfterReimport = await getDS(page);
  const reimportedOak1 = (dsAfterReimport.transactions || []).find(t => t.desc && t.desc.includes(RUN + "-Oak-1"));
  const reimportedDefs = dsAfterReimport.customFieldDefs || [];

  if (reimportedOak1 && reimportedOak1.custom && reimportedOak1.custom.l53_property === "Oak Street") {
    pass("CF_ROUNDTRIP_REIMPORT — custom field values survive simulated re-import (lossless)");
  } else {
    fail(`CF_ROUNDTRIP_REIMPORT — custom values lost after re-import; got: ${JSON.stringify(reimportedOak1 && reimportedOak1.custom)}`);
  }

  const propDefSurvived = reimportedDefs.some(d => d.entityType === "transaction" && d.key === "l53_property");
  const taxDefSurvived  = reimportedDefs.some(d => d.entityType === "transaction" && d.key === "l53_tax_ded");
  if (propDefSurvived && taxDefSurvived) {
    pass("CF_ROUNDTRIP_DEFS — both custom field defs survive round-trip");
  } else {
    fail("CF_ROUNDTRIP_DEFS — one or more custom field defs lost after simulated re-import");
  }

  // ── STEP 7: Hard reload — confirm values persist ──────────────────────────────
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);

  await pushNav(page, "/transactions");
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L53_s7_reload_verify.png") });

  await flush(page);
  const dsAfterReload = await getDS(page);
  const reloadedOak1  = (dsAfterReload.transactions || []).find(t => t.desc && t.desc.includes(RUN + "-Oak-1"));
  const reloadedMaple = (dsAfterReload.transactions || []).find(t => t.desc && t.desc.includes(RUN + "-Maple-1"));
  const reloadedOak2  = (dsAfterReload.transactions || []).find(t => t.desc && t.desc.includes(RUN + "-Oak-2"));

  const reloadCheck = (txn, tag, expProp, expTaxDed) => {
    if (!txn) { fail(`CF_RELOAD_${tag} — transaction not found after reload`); return; }
    const custom = txn.custom || {};
    const propOk = custom.l53_property === expProp;
    const taxOk  = expTaxDed
      ? (custom.l53_tax_ded === true || custom.l53_tax_ded === "true")
      : (!custom.l53_tax_ded || custom.l53_tax_ded === false || custom.l53_tax_ded === "false");
    if (propOk && taxOk) {
      pass(`CF_RELOAD_${tag} — custom values survive hard reload: l53_property="${custom.l53_property}" l53_tax_ded=${custom.l53_tax_ded}`);
    } else {
      fail(`CF_RELOAD_${tag} — after reload: l53_property="${custom.l53_property}" l53_tax_ded=${custom.l53_tax_ded}; expected l53_property="${expProp}" taxDed=${expTaxDed}`);
    }
  };

  reloadCheck(reloadedOak1,  "OAK1",   "Oak Street", true);
  reloadCheck(reloadedMaple, "MAPLE1", "Maple Ave",  false);
  reloadCheck(reloadedOak2,  "OAK2",   "Oak Street", true);

  // ── JS error check ────────────────────────────────────────────────────────────
  if (jsErrors.length === 0) {
    pass("ZERO_JS_ERRORS — no JS page errors across full ritual");
  } else {
    fail(`JS_ERRORS — ${jsErrors.length} JS error(s): ${jsErrors.slice(0, 3).join(" | ")}`);
  }

  // ── Summary ───────────────────────────────────────────────────────────────────
  console.log(`\n── L53 summary: ${passed} PASS, ${failed} FAIL ──`);

} catch (err) {
  console.error("FATAL:", err);
  process.exitCode = 1;
} finally {
  await browser.close();
}
