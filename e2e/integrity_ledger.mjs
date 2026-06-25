// Ledger integrity e2e — the core invariant of a finance app: a transaction must
// move the affected account's balance AND household net worth by EXACTLY its amount,
// in the right direction (expense ↓, income ↑). Catches double-counting, sign, and
// net-worth-aggregation regressions.
//
// Runs with reducedMotion so the count-up flourish (countup.js tweens .fig from 0)
// doesn't poison figure reads — the figures render at their final value immediately.
//
// Run: node e2e/integrity_ledger.mjs   (against `go run e2e/serve.go` on :8099)
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TARGET = "Roth IRA"; // a stable, net-worth-included asset account in the sample
const AMT = 50;

let passed = 0, failed = 0;
const pass = (l) => { console.log("PASS:   " + l); passed++; };
const fail = (l) => { console.error("FAIL:   " + l); failed++; };
const note = (l) => console.log("NOTE:   " + l);
const num = (s) => s == null ? null : parseFloat(String(s).replace(/[()]/g, (m) => m === "(" ? "-" : "").replace(/[^0-9.\-]/g, ""));

const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({ reducedMotion: "reduce" });
const jsErrors = [];
try {
  const p = await ctx.newPage();
  p.setViewportSize({ width: 1280, height: 950 });
  p.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  const nav = async (t) => { await p.evaluate((t) => { const l = [...document.querySelectorAll('nav a[title]')].find((x) => x.getAttribute("title") === t); if (l) l.click(); }, t); await p.waitForTimeout(1500); };
  const readNW = async () => {
    for (let i = 0; i < 6; i++) {
      const v = num(await p.evaluate(() => document.querySelector('.home-hero-nw-fig')?.textContent.trim() || null));
      if (v != null) return v;
      await p.waitForTimeout(400);
    }
    return null;
  };
  const readAcct = (name) => p.evaluate((name) => {
    const r = [...document.querySelectorAll('.row')].find((x) => (x.textContent || "").includes(name) && !/restore/i.test(x.textContent || ""));
    if (!r) return null;
    const ds = [...(r.textContent || "").matchAll(/\$([\d,]+\.?\d*)/g)].map((m) => parseFloat(m[1].replace(/,/g, "")));
    return ds.length ? ds[ds.length - 1] : null;
  }, name);
  // Add a transaction via the dashboard quick-add modal. kind = "Expense" | "Income".
  const addTxn = async (kind, amount, name) => {
    await p.evaluate(() => { const b = [...document.querySelectorAll('button')].find((b) => /add transaction/i.test(b.textContent || "")); if (b) b.click(); });
    await p.waitForTimeout(700);
    await p.evaluate(({ kind, name }) => {
      // Scope to the modal's segmented toggle (.seg-btn) — a bare button match also
      // hits the dashboard's "Income" KPI widget header behind the modal.
      const t = [...document.querySelectorAll('.seg-btn')].find((b) => new RegExp("^" + kind + "$", "i").test((b.textContent || "").trim())); if (t) t.click();
      const sel = document.querySelector('[data-testid="txn-add-account"]');
      if (sel) { const opt = [...sel.options].find((o) => o.textContent.includes(name)); if (opt) sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true })); }
    }, { kind, name });
    await p.fill('[data-testid="txn-add-amount"]', String(amount));
    await p.fill('[data-testid="txn-add-desc"]', "QA ledger integrity");
    await p.evaluate(() => { const s = [...document.querySelectorAll('button')].find((b) => /^save$/i.test((b.textContent || "").trim())); if (s) s.click(); });
    await p.waitForTimeout(1500);
  };

  await p.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await p.evaluate(() => { const b = [...document.querySelectorAll("button")].find((b) => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await p.waitForTimeout(1800);
  pass("HYDRATION — app booted + sample loaded");

  const nw0 = await readNW();
  await nav("Accounts"); const bal0 = await readAcct(TARGET); await nav("Dashboard");
  note(`baseline — net worth $${nw0}, ${TARGET} $${bal0}`);
  if (nw0 == null || bal0 == null) { fail("baseline unreadable (nw=" + nw0 + ", acct=" + bal0 + ")"); throw new Error("baseline"); }

  // EXPENSE: balance ↓ AMT, net worth ↓ AMT
  await addTxn("Expense", AMT, TARGET);
  let nw1 = await readNW();
  await nav("Accounts"); let bal1 = await readAcct(TARGET); await nav("Dashboard");
  note(`after expense $${AMT} — net worth $${nw1}, ${TARGET} $${bal1}`);
  Math.abs((bal0 - bal1) - AMT) < 0.01 ? pass(`A-1 expense ↓ account balance by EXACTLY $${AMT} ($${bal0} → $${bal1})`) : fail(`A-1 balance delta ${(bal1 - bal0).toFixed(2)} != -${AMT}`);
  Math.abs((nw0 - nw1) - AMT) < 0.01 ? pass(`A-2 expense ↓ net worth by EXACTLY $${AMT} ($${nw0} → $${nw1})`) : fail(`A-2 net-worth delta ${(nw1 - nw0).toFixed(2)} != -${AMT}`);

  // INCOME: balance ↑ AMT, net worth ↑ AMT (round-trips back to baseline)
  await addTxn("Income", AMT, TARGET);
  let nw2 = await readNW();
  await nav("Accounts"); let bal2 = await readAcct(TARGET); await nav("Dashboard");
  note(`after income $${AMT} — net worth $${nw2}, ${TARGET} $${bal2}`);
  Math.abs((bal2 - bal1) - AMT) < 0.01 ? pass(`A-3 income ↑ account balance by EXACTLY $${AMT} ($${bal1} → $${bal2})`) : fail(`A-3 balance delta ${(bal2 - bal1).toFixed(2)} != +${AMT}`);
  Math.abs((nw2 - nw1) - AMT) < 0.01 ? pass(`A-4 income ↑ net worth by EXACTLY $${AMT} ($${nw1} → $${nw2})`) : fail(`A-4 net-worth delta ${(nw2 - nw1).toFixed(2)} != +${AMT}`);
  Math.abs(bal2 - bal0) < 0.01 && Math.abs(nw2 - nw0) < 0.01 ? pass("A-5 expense+income round-trips back to baseline (no drift)") : fail(`A-5 drift: bal ${bal2} vs ${bal0}, nw ${nw2} vs ${nw0}`);

  jsErrors.length === 0 ? pass("A-6 zero runtime JS errors across the flow") : fail("A-6 " + jsErrors.length + " JS errors: " + jsErrors.slice(0, 3).join("; "));
} catch (err) {
  if (String(err.message) !== "baseline") { fail("UNEXPECTED_ERROR — " + err.message); console.error(err); }
} finally { await browser.close(); }
console.log("\n════════════════════════════════════════════");
console.log("RESULT: " + passed + " PASS · " + failed + " FAIL");
console.log("════════════════════════════════════════════");
process.exit(failed > 0 ? 1 : 0);
