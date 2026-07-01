// e2e for the app-lock screen additions: a music MUTE toggle, and the bottom
// verbiage using the Smart+ "quote of the day" engine (with a static fallback).
//
// Run: node e2e/lockscreen_check.mjs  (against `go run e2e/serve.go` on :8099)
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
async function seedIDB(p, k, v) { await p.evaluate(async ([k, v]) => { const db = await new Promise((res) => { const r = indexedDB.open("cashflux-kv"); r.onsuccess = () => res(r.result); }); await new Promise((res) => { const t = db.transaction("kv", "readwrite"); t.objectStore("kv").put(v, k); t.oncomplete = () => res(); }); }, [k, v]); }
async function setPasscode(p) {
  await p.locator(".bento-accounts .row button[aria-haspopup='menu']").first().click(); await p.waitForTimeout(300);
  await p.locator('.add-menu:not(.hidden-menu) [data-testid^="creds-start-btn-"]').first().click(); await p.waitForTimeout(700);
  await p.locator('.flip-back button:has-text("Open Settings")').first().click(); await p.waitForTimeout(1200);
  await p.locator('button:has-text("Set passcode")').first().waitFor({ state: "visible", timeout: 10000 }).catch(() => {});
  await p.locator('button:has-text("Set passcode")').first().click(); await p.waitForTimeout(400);
  await p.fill("#cf-al-pass", "7Km9Qz2p"); await p.fill("#cf-al-confirm", "7Km9Qz2p"); await p.click("#cf-al-ok"); await p.waitForTimeout(900);
}
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) errs.push(m); });
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" }); await ready(page);
  await setPasscode(page);

  // Seed the Smart+ quote-of-the-day (enabled + a cached result) — lives in its own
  // browserstore key, so it's readable on the lock screen even while locked.
  await seedIDB(page, "cashflux:smart-settings", JSON.stringify({ enabled: { "SMART-QUOTE": true }, results: { "SMART-QUOTE": "AIQUOTE Beware of little expenses — Benjamin Franklin" } }));
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" }); await page.waitForTimeout(2200);
  check("L0 lock gate shown on reload (passcode set)", await page.evaluate(() => !!document.getElementById("cf-applock-input")));
  const q1 = await page.evaluate(() => document.getElementById("cf-lock-quote")?.textContent || "");
  check("L1 lock quote uses the Smart+ quote-of-the-day when enabled", q1.includes("AIQUOTE"), q1.slice(0, 60));

  // Mute button present + toggles the music.
  const mb = await page.evaluate(() => ({ exists: !!document.getElementById("cf-lock-mute"), size: (window.cashfluxMuzak?.state?.() || {}).size }));
  check("L2 mute button present in the lock gate", mb.exists, JSON.stringify(mb));
  await page.evaluate(() => window.cashfluxMuzak?.setEnabled(true)); await page.waitForTimeout(200);
  const wasOn = await page.evaluate(() => window.cashfluxMuzak?.isEnabled?.());
  await page.locator("#cf-lock-mute").click(); await page.waitForTimeout(400);
  const nowOff = await page.evaluate(() => window.cashfluxMuzak?.isEnabled?.());
  check("L3 mute button toggles the music off", wasOn === true && nowOff === false, `on=${wasOn} off=${nowOff}`);

  // Disable the quote feature → static fallback verbiage.
  await page.fill("#cf-applock-input", "7Km9Qz2p"); await page.locator("button:has-text('Unlock')").first().click(); await page.waitForTimeout(1500);
  await seedIDB(page, "cashflux:smart-settings", JSON.stringify({ enabled: {}, explicitOff: { "SMART-QUOTE": true } }));
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" }); await page.waitForTimeout(2200);
  const q2 = await page.evaluate(() => document.getElementById("cf-lock-quote")?.textContent || "");
  check("L4 falls back to a static quote when the AI quote is disabled", q2.length > 0 && !q2.includes("AIQUOTE"), q2.slice(0, 60));

  const pass = results.filter(r => r.ok).length, fail = results.length - pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if (fail) { console.log("FAILED: " + results.filter(r => !r.ok).map(r => r.n).join(", ")); process.exitCode = 1; }
  console.log("page errors: " + (errs.length ? JSON.stringify([...new Set(errs)].slice(0, 6)) : "none"));
  console.log("════════════════════════════════════════════");
} finally { await browser.close(); }
