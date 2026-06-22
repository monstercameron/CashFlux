// L12 gate — subscription cancellation tracking + charged-after-cancel alert.
//
// Seeded subscription used: "Gym membership" (Iron Works Gym, $40 on day 3 of
// every sample month through Jun 3 2026), detected by subscriptions.Detect.
//
// Strategy: a cancellation dated BEFORE a real charge is what triggers the
// alert, but the UI's "Mark as cancelled" stamps *today* (no past charge after
// it). So we inject a SubscriptionCancellation{SubName:"Gym membership",
// CancelledOn:2026-05-20} — before the Jun 3 2026 charge. A plain localStorage
// write loses to the reloading page's pagehide->autosave, so a one-shot
// addInitScript injects it at document-start (after that save, before wasm boot
// reads localStorage) and consumes its sentinel so it runs only once. We then
// assert (a) the cancellation persists in cashflux:dataset and (b) the
// charged-after-cancel alert renders, plus that the "Mark as cancelled" action
// is present (the action is wired).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SUB = "Gym membership";
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

  await page.goto(BASE + "/subscriptions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("h1, h2", { timeout: 60000 });
  // The detected sub + its "Mark as cancelled" action are present.
  await page.waitForSelector(`[aria-label*="${SUB}"]`, { timeout: 30000 });

  // Wait for the seeded dataset to persist so the init script has something to
  // inject into on reload.
  await waitDS(page, (d) => Array.isArray(d.transactions) && d.transactions.length > 0);

  // Arm the one-shot injection, then reload.
  await page.evaluate((sub) => localStorage.setItem("e2e-inject-cancel", sub), SUB);
  await page.addInitScript(() => {
    const sub = localStorage.getItem("e2e-inject-cancel");
    if (!sub) return;
    localStorage.removeItem("e2e-inject-cancel"); // one-shot
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.subscriptionCancellations = ds.subscriptionCancellations || [];
      if (!ds.subscriptionCancellations.some((c) => c.subName === sub)) {
        ds.subscriptionCancellations.push({ id: "e2e-cancel-1", subName: sub, cancelledOn: "2026-05-20T00:00:00Z" });
      }
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("h1, h2", { timeout: 60000 });

  // (a) The cancellation persists.
  const d = await waitDS(page, (dd) => (dd.subscriptionCancellations || []).some((c) => c.subName === SUB));
  if (!(d.subscriptionCancellations || []).some((c) => c.subName === SUB)) {
    fail("subscriptionCancellations entry did not persist for " + SUB);
  }

  // (b) The charged-after-cancel alert renders and names the sub.
  const alert = page.locator('[role="alert"]', { hasText: SUB });
  const seen = await alert.first().isVisible().catch(() => false);
  if (!seen) fail(`charged-after-cancel alert mentioning "${SUB}" not visible`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: "${SUB}" cancellation persisted and the charged-after-cancel alert is shown.`);
} finally {
  await browser.close();
}
