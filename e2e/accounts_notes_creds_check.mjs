// e2e for account Notes + the encrypted institution-credential vault.
//
// Verifies (a) a Notes field that persists + shows a row indicator, and (b) the
// credential vault: a security warning + passcode gate, then (after setting an app
// passcode) that credentials encrypt to a DEDICATED local-only key, contain no
// plaintext, are NEVER in the dataset blob (so never exported/synced), and decrypt
// back on reopen. Reads IndexedDB (cashflux-kv), not the stale localStorage.
//
// Run: node e2e/accounts_notes_creds_check.mjs  (against `go run e2e/serve.go` on :8099)
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const results = []; let errs = [];
function check(n, c, d = "") { results.push({ n, ok: !!c }); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); }
async function ready(p) { await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {}); await p.waitForFunction(() => { const b = document.getElementById("boot"); return !b || b.classList.contains("hidden") || b.offsetParent === null; }, { timeout: 15000 }).catch(() => {}); await p.waitForTimeout(500); }
async function flush(p) { await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await p.waitForTimeout(500); }
async function idbKey(p, key) { return await p.evaluate(async (k) => { const db = await new Promise((res, rej) => { const r = indexedDB.open("cashflux-kv"); r.onsuccess = () => res(r.result); r.onerror = () => rej(r.error); }); const tx = db.transaction("kv", "readonly"); const v = await new Promise((res) => { const r = tx.objectStore("kv").get(k); r.onsuccess = () => res(r.result); }); return v || ""; }, key); }
async function dataset(p) { await flush(p); const v = await idbKey(p, "cashflux:dataset"); try { return JSON.parse(v || "{}"); } catch (e) { return {}; } }

try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) errs.push(m); });
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" }); await ready(page);

  // ── Notes ──────────────────────────────────────────────────────────────
  await page.locator(".bento-accounts .row").first().locator('button:has-text("Edit")').first().click();
  await page.waitForTimeout(700);
  check("N1 edit modal has a Notes textarea", await page.evaluate(() => !!document.querySelector(".acct-edit-form textarea")));
  await page.locator(".acct-edit-form textarea").first().fill("Joint account with mum — branch on 5th St");
  await page.locator(".acct-edit-actions button[type='submit']").first().click();
  await page.waitForTimeout(700);
  const ds = await dataset(page);
  check("N2 notes persist to the account (IDB)", (ds.accounts || []).some(a => (a.notes || "").includes("Joint account with mum")));
  check("N3 row shows a notes indicator", await page.evaluate(() => !!document.querySelector("[data-testid^='acct-notes-dot-']")));

  // ── Credential gate (no passcode) ──────────────────────────────────────
  await page.locator(".bento-accounts .row button[aria-haspopup='menu']").first().click(); await page.waitForTimeout(300);
  await page.locator('.add-menu:not(.hidden-menu) [data-testid^="creds-start-btn-"]').first().click();
  await page.waitForTimeout(700);
  const gate = await page.evaluate(() => ({ modal: !!document.querySelector(".flip-wrap"), warn: !!document.querySelector("[data-testid='cred-security-warning']"), needPass: /passcode/i.test(document.querySelector(".flip-back")?.textContent || "") }));
  check("C1 credential modal opens with security warning", gate.modal && gate.warn);
  check("C2 gated behind a passcode when none set", gate.needPass);
  await page.locator('.flip-back button:has-text("Open Settings")').first().click();
  await page.waitForTimeout(1200);

  // ── Set an app passcode ────────────────────────────────────────────────
  const setBtn = page.locator('button:has-text("Set passcode")');
  await setBtn.first().waitFor({ state: "visible", timeout: 10000 }).catch(() => {});
  check("C3 App-lock 'Set passcode' reachable in Settings", (await setBtn.count()) > 0);
  await setBtn.first().click();
  await page.waitForTimeout(400);
  check("C4 passcode setup modal opens", await page.evaluate(() => !!document.getElementById("cf-al-pass")));
  await page.fill("#cf-al-pass", "7Km9Qz2p");
  await page.fill("#cf-al-confirm", "7Km9Qz2p");
  await page.click("#cf-al-ok");
  await page.waitForTimeout(900);
  const setupState = await page.evaluate(() => { const el = document.getElementById("cf-al-pass"); return { hidden: !el || el.offsetParent === null, err: document.getElementById("cf-al-err")?.textContent || "" }; });
  check("C5 passcode set (setup dismissed)", setupState.hidden, setupState.err ? "err=" + setupState.err : "");
  // stay in-session (a reload would re-lock); close settings, reopen creds via ⋯
  await page.keyboard.press("Escape"); await page.waitForTimeout(400);
  await page.keyboard.press("Escape"); await page.waitForTimeout(400);
  await page.locator(".bento-accounts .row button[aria-haspopup='menu']").first().click({ timeout: 15000 }); await page.waitForTimeout(300);
  await page.locator('.add-menu:not(.hidden-menu) [data-testid^="creds-start-btn-"]').first().click();
  await page.waitForTimeout(800);
  const formShown = await page.evaluate(() => !!document.querySelector("[data-testid='cred-username']"));
  check("C6 credential form available after passcode set", formShown);
  if (formShown) {
    await page.fill("[data-testid='cred-username']", "acct_holder_42");
    await page.fill("[data-testid='cred-password']", "S3cretBankPw!x");
    await page.fill("[data-testid='cred-url']", "https://mybank.example/login");
    await page.fill(".acct-edit-form textarea", "Security Q: first pet = Rex");
    const tBefore = await page.evaluate(() => document.querySelector("[data-testid='cred-password']").type);
    await page.locator("[data-testid='cred-reveal']").click(); await page.waitForTimeout(200);
    const tAfter = await page.evaluate(() => document.querySelector("[data-testid='cred-password']").type);
    check("C7 reveal toggles password visibility", tBefore === "password" && tAfter === "text");
    await page.locator("[data-testid='cred-save']").click();
    await page.waitForTimeout(900); await flush(page);
    const vault = await idbKey(page, "cashflux:credvault");
    const dsAfter = await idbKey(page, "cashflux:dataset");
    check("C8 credential vault written to its own key", vault.length > 0);
    check("C9 vault ciphertext contains NO plaintext secret", !/S3cretBankPw|acct_holder_42|first pet = Rex/.test(vault), "len=" + vault.length);
    check("C10 credentials are NOT in the dataset blob (never exported/synced)", !/S3cretBankPw|acct_holder_42|mybank.example/.test(dsAfter));
    await page.locator(".bento-accounts .row button[aria-haspopup='menu']").first().click(); await page.waitForTimeout(300);
    await page.locator('.add-menu:not(.hidden-menu) [data-testid^="creds-start-btn-"]').first().click();
    await page.waitForTimeout(900);
    const reloaded = await page.evaluate(() => ({ u: document.querySelector("[data-testid='cred-username']")?.value, url: document.querySelector("[data-testid='cred-url']")?.value }));
    check("C11 reopening decrypts + shows saved credentials", reloaded.u === "acct_holder_42" && /mybank.example/.test(reloaded.url || ""), JSON.stringify(reloaded));
  }

  const pass = results.filter(r => r.ok).length, fail = results.length - pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if (fail) { console.log("FAILED: " + results.filter(r => !r.ok).map(r => r.n).join(", ")); process.exitCode = 1; }
  console.log("page errors: " + (errs.length ? JSON.stringify([...new Set(errs)].slice(0, 6)) : "none"));
  console.log("════════════════════════════════════════════");
} finally { await browser.close(); }
