// L24 gate — "due recurring transactions auto-post the moment the app opens."
// Backdates a seeded autopost recurring (rec-salary, monthly) to a past NextDue,
// reloads, and asserts the boot auto-post created the caught-up transaction(s)
// and advanced the schedule's NextDue past today (idempotent — no double-post).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getDS = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitDS(page, pred, timeoutMs = 10000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    d = await getDS(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  // Wait for the seeded dataset (with the recurring schedules) to persist.
  const d0 = await waitDS(page, (d) => (d.recurring || []).some((r) => r.id === "rec-salary"));
  const rec = (d0.recurring || []).find((r) => r.id === "rec-salary");
  if (!rec) { fail("seeded rec-salary autopost recurring not found"); process.exit(1); }
  if (!rec.autopost) { fail("rec-salary is not an autopost recurring"); process.exit(1); }

  const labelCountBefore = (d0.transactions || []).filter((t) => t.desc === rec.label).length;

  // Backdate NextDue ~2 months back so boot auto-post catches up at least one
  // occurrence. The edit must land at document-start on the NEXT load: the
  // reloading page fires pagehide → autosave, which would otherwise clobber a
  // plain localStorage edit with the in-memory (future) NextDue. A one-shot
  // init script (consumes its sentinel) backdates after that save but before the
  // wasm boot reads localStorage.
  await page.evaluate(() => localStorage.setItem("e2e-backdate-recsalary", "2026-04-15T00:00:00Z"));
  await page.addInitScript(() => {
    const when = localStorage.getItem("e2e-backdate-recsalary");
    if (!when) return;
    localStorage.removeItem("e2e-backdate-recsalary"); // one-shot
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      const r = (ds.recurring || []).find((x) => x.id === "rec-salary");
      if (r) { r.nextDue = when; localStorage.setItem("cashflux:dataset", JSON.stringify(ds)); }
    } catch (e) { /* ignore */ }
  });

  // Reload — boot should auto-post the caught-up paychecks.
  await page.reload({ waitUntil: "domcontentloaded" });
  const after = await waitDS(page, (d) => {
    const cnt = (d.transactions || []).filter((t) => t.desc === rec.label).length;
    const r = (d.recurring || []).find((x) => x.id === "rec-salary");
    return cnt > labelCountBefore && r && new Date(r.nextDue) > new Date("2026-06-21T00:00:00Z");
  });

  const labelCountAfter = (after.transactions || []).filter((t) => t.desc === rec.label).length;
  const recAfter = (after.recurring || []).find((x) => x.id === "rec-salary");
  if (labelCountAfter <= labelCountBefore) {
    fail(`boot did not auto-post the due recurring (before=${labelCountBefore}, after=${labelCountAfter})`);
  }
  if (!recAfter || new Date(recAfter.nextDue) <= new Date("2026-06-21T00:00:00Z")) {
    fail(`NextDue was not advanced past today (got ${recAfter && recAfter.nextDue})`);
  }

  // Idempotency: a second reload must NOT post again.
  await page.reload({ waitUntil: "domcontentloaded" });
  const after2 = await waitDS(page, (d) => (d.recurring || []).some((r) => r.id === "rec-salary"));
  const cnt2 = (after2.transactions || []).filter((t) => t.desc === rec.label).length;
  if (cnt2 !== labelCountAfter) {
    fail(`reopen double-posted (after=${labelCountAfter}, after2=${cnt2}) — not idempotent`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(`PASS: boot auto-posted ${labelCountAfter - labelCountBefore} due "${rec.label}" txn(s), advanced NextDue to ${recAfter.nextDue}, and did not double-post on reopen.`);
  }
} finally {
  await browser.close();
}
