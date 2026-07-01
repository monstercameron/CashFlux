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
  // Ensure the dataset has at least one account so the passcode-setup path (which
  // goes through an account's ⋯ menu) has a row to click — a fresh origin is empty.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" }); await ready(page);
  if (await page.locator('[data-testid="hero-load-sample"]').count()) {
    await page.locator('[data-testid="hero-load-sample"]').click(); await page.waitForTimeout(1800);
  }
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" }); await ready(page);

  // With no passcode set (lock disabled), the lock button must NOT be in the DOM —
  // only its empty (display:none) wrapper slot holds the position.
  const k0 = await page.evaluate(() => ({
    btnInDom: !!document.querySelector('[data-testid="topbar-lock-btn"]'),
    slotHidden: (() => { const s = document.querySelector(".tb-actions .lock-toggle-slot"); return !!s && getComputedStyle(s).display === "none"; })(),
  }));
  check("K0 lock button absent from DOM when the lock is disabled", !k0.btnInDom && k0.slotHidden, JSON.stringify(k0));

  await setPasscode(page);
  // Dismiss any lingering flip modal/backdrop from the passcode-setup flow so it
  // doesn't intercept the topbar clicks below.
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForFunction(() => !document.querySelector(".flip-backdrop.show"), { timeout: 5000 }).catch(() => {});
  await page.waitForTimeout(400);

  // Top-bar lock button: appears once a passcode is set, sits immediately beside the
  // music (mute) toggle, and locks the app on click. Checked while unlocked (topbar visible).
  const tb = await page.evaluate(() => {
    const acts = document.querySelector(".tb-actions");
    const kids = acts ? [...acts.children] : [];
    const mzIdx = kids.findIndex((c) => c.classList.contains("muzak-btn") && !c.getAttribute("data-testid"));
    // The lock button is conditionally rendered inside its stable wrapper slot.
    const slotIdx = kids.findIndex((c) => c.classList.contains("lock-toggle-slot"));
    const slot = slotIdx >= 0 ? kids[slotIdx] : null;
    const hasLockBtn = !!slot && !!slot.querySelector('[data-testid="topbar-lock-btn"]');
    return { hasLock: hasLockBtn, adjacent: slotIdx >= 0 && slotIdx === mzIdx + 1, mzIdx, slotIdx };
  });
  check("K1 top-bar lock button appears when a passcode is set", tb.hasLock, JSON.stringify(tb));
  check("K2 lock button sits immediately beside the music toggle", tb.adjacent, JSON.stringify(tb));
  await page.locator('[data-testid="topbar-lock-btn"]').click(); await page.waitForTimeout(600);
  check("K3 clicking the lock button locks the app (gate shown)", await page.evaluate(() => !!document.getElementById("cf-applock-input")));
  // Unlock again so the rest of the checks start from a known (unlocked) state.
  await page.fill("#cf-applock-input", "7Km9Qz2p"); await page.locator("button:has-text('Unlock')").first().click(); await page.waitForTimeout(1200);

  // Seed a cached Smart+ quote-of-the-day WITHOUT the manual dashboard opt-in — this is
  // exactly what refreshDailyLockQuote writes after just an AI key is configured. The
  // cache lives in its own browserstore key, readable on the lock screen while locked.
  // The fix: the lock screen shows any cached quote (unless explicitly turned off), so
  // "added a key" surfaces the AI quote instead of the static fallback.
  await seedIDB(page, "cashflux:smart-settings", JSON.stringify({ enabled: {}, results: { "SMART-QUOTE": "AIQUOTE Beware of little expenses — Benjamin Franklin" } }));
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" }); await page.waitForTimeout(2200);
  check("L0 lock gate shown on reload (passcode set)", await page.evaluate(() => !!document.getElementById("cf-applock-input")));
  const q1 = await page.evaluate(() => document.getElementById("cf-lock-quote")?.textContent || "");
  check("L1 lock quote shows the cached AI quote-of-the-day (no manual opt-in needed)", q1.includes("AIQUOTE"), q1.slice(0, 60));

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
