// e2e for account Notes + the encrypted institution-credential vault.
//
// Verifies (a) a Notes field that persists + shows a row indicator, and (b) the
// credential vault: a security warning + passcode gate; after setting an app passcode,
// credentials encrypt to a DEDICATED local-only key with no plaintext and are NEVER in
// the dataset blob; and retrieval is copy-to-clipboard behind a passcode re-auth — the
// stored password is never pre-filled, never rendered, never anywhere in the DOM; plus
// a login-page quick link. Reads IndexedDB (cashflux-kv), not the stale localStorage.
//
// Run: node e2e/accounts_notes_creds_check.mjs  (against `go run e2e/serve.go` on :8099)
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PW = "S3cretBankPw!x", USER = "acct_holder_42", URL = "mybank.example/login";
const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, permissions: ["clipboard-read", "clipboard-write"] });
const results = []; let errs = [];
function check(n, c, d = "") { results.push({ n, ok: !!c }); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); }
async function ready(p) { await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {}); await p.waitForFunction(() => { const b = document.getElementById("boot"); return !b || b.classList.contains("hidden") || b.offsetParent === null; }, { timeout: 15000 }).catch(() => {}); await p.waitForTimeout(500); }
async function flush(p) { await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await p.waitForTimeout(500); }
async function idbKey(p, key) { return await p.evaluate(async (k) => { const db = await new Promise((res, rej) => { const r = indexedDB.open("cashflux-kv"); r.onsuccess = () => res(r.result); r.onerror = () => rej(r.error); }); const tx = db.transaction("kv", "readonly"); const v = await new Promise((res) => { const r = tx.objectStore("kv").get(k); r.onsuccess = () => res(r.result); }); return v || ""; }, key); }
async function dataset(p) { await flush(p); const v = await idbKey(p, "cashflux:dataset"); try { return JSON.parse(v || "{}"); } catch (e) { return {}; } }
async function openCreds(p) { await p.locator(".bento-accounts .row button[aria-haspopup='menu']").first().click(); await p.waitForTimeout(300); await p.locator('.add-menu:not(.hidden-menu) [data-testid^="creds-start-btn-"]').first().click(); await p.waitForTimeout(800); }

try {
  const page = await ctx.newPage();
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
  await openCreds(page);
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
  check("C5 passcode set (setup dismissed)", await page.evaluate(() => { const el = document.getElementById("cf-al-pass"); return !el || el.offsetParent === null; }));
  await page.keyboard.press("Escape"); await page.waitForTimeout(400);
  await page.keyboard.press("Escape"); await page.waitForTimeout(400);

  // ── Enter + save credentials ───────────────────────────────────────────
  await openCreds(page);
  check("C6 credential form available after passcode set", await page.evaluate(() => !!document.querySelector("[data-testid='cred-username']")));
  await page.fill("[data-testid='cred-username']", USER);
  await page.fill("[data-testid='cred-password']", PW);
  await page.fill("[data-testid='cred-url']", "https://" + URL);
  await page.locator("[data-testid='cred-save']").click();
  await page.waitForTimeout(900); await flush(page);
  const vault = await idbKey(page, "cashflux:credvault");
  const dsAfter = await idbKey(page, "cashflux:dataset");
  check("C7 credential vault written to its own key", vault.length > 0);
  check("C8 vault ciphertext contains NO plaintext secret", !new RegExp(PW + "|" + USER).test(vault), "len=" + vault.length);
  check("C9 credentials are NOT in the dataset blob (never exported/synced)", !new RegExp(PW + "|" + USER + "|mybank.example").test(dsAfter));

  // ── Retrieval redesign: no DOM, quick link, copy-behind-reauth ─────────
  await openCreds(page);
  const state = await page.evaluate((pw) => ({
    passInputVal: document.querySelector("[data-testid='cred-password']")?.value,
    passInDom: (document.documentElement.outerHTML || "").includes(pw),
    reveal: !!document.querySelector("[data-testid='cred-reveal']"),
    copyBtn: !!document.querySelector("[data-testid='cred-copy']"),
    loginLink: document.querySelector("[data-testid='cred-open-login']")?.getAttribute("href"),
    userVal: document.querySelector("[data-testid='cred-username']")?.value,
  }), PW);
  check("C10 stored password NOT pre-filled into the input", state.passInputVal === "");
  check("C11 password string is NOT anywhere in the DOM", state.passInDom === false);
  check("C12 no reveal toggle (never shown)", state.reveal === false);
  check("C13 username still shown/editable", state.userVal === USER);
  check("C14 Copy-password button present", state.copyBtn);
  check("C15 login-page quick link present + points at the URL", (state.loginLink || "").includes(URL), state.loginLink);
  // wrong passcode rejected
  await page.locator("[data-testid='cred-copy']").click(); await page.waitForTimeout(500);
  check("C16 Copy triggers a passcode re-auth prompt", await page.evaluate(() => !!document.getElementById("cf-cred-reauth")));
  await page.fill("#cf-cred-reauth", "0000wrong"); await page.click("#cf-cred-reauth-ok"); await page.waitForTimeout(400);
  check("C17 wrong passcode rejected (no copy)", await page.evaluate(() => !!document.getElementById("cf-cred-reauth") && /incorrect/i.test(document.getElementById("cf-cred-reauth-err")?.textContent || "")));
  // correct passcode copies to clipboard
  await page.evaluate(() => navigator.clipboard.writeText("__cleared__"));
  await page.fill("#cf-cred-reauth", "7Km9Qz2p"); await page.click("#cf-cred-reauth-ok"); await page.waitForTimeout(700);
  check("C18 correct passcode copies the password to the clipboard", (await page.evaluate(() => navigator.clipboard.readText())) === PW);
  check("C19 password STILL not in the DOM after copy", await page.evaluate((pw) => !(document.documentElement.outerHTML || "").includes(pw), PW));

  const pass = results.filter(r => r.ok).length, fail = results.length - pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if (fail) { console.log("FAILED: " + results.filter(r => !r.ok).map(r => r.n).join(", ")); process.exitCode = 1; }
  console.log("page errors: " + (errs.length ? JSON.stringify([...new Set(errs)].slice(0, 6)) : "none"));
  console.log("════════════════════════════════════════════");
} finally { await browser.close(); }
