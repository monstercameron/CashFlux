// L63 GAP-A — custom page "Bills" list source.
// Injects a liability account with a due date so it generates an upcoming bill,
// then creates a custom page with a List widget bound to "bills" and verifies
// the bill's name appears in the widget body (not a "Pick a data source" error).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const TS = Date.now();
const BILL_NAME = "TestVisa_" + TS;
const PAGE_SLUG = "billtest" + TS;

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });

  // Stash the injection payload, then re-apply it at document-start on the next
  // navigation via a one-shot addInitScript — the goto below fires pagehide →
  // autosave on the current page, which would otherwise clobber a plain
  // localStorage edit with the in-memory dataset before the new page's wasm boots.
  await page.evaluate(([name, slug, ts]) => {
    localStorage.setItem("e2e-billsrc", JSON.stringify({ name, slug, ts }));
  }, [BILL_NAME, PAGE_SLUG, TS]);
  await page.addInitScript(() => {
    const raw = localStorage.getItem("e2e-billsrc");
    if (!raw) return;
    localStorage.removeItem("e2e-billsrc"); // one-shot
    try {
      const { name, slug, ts } = JSON.parse(raw);
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.accounts = ds.accounts || [];
      ds.accounts.push({
        id: "acc_bill_" + ts, name, type: "credit", class: "liability", currency: "USD",
        dueDayOfMonth: 28, minPayment: { Amount: 2500, Currency: "USD" },
        openingBalance: { Amount: 0, Currency: "USD" }, archived: false,
      });
      ds.customPages = ds.customPages || [];
      ds.customPages.push({
        id: "pg_bill_" + ts, title: "Bills Test", slug, hidden: false,
        widgets: [{ id: "w_bill_" + ts, type: "list", title: "Upcoming Bills", binding: { source: "bills" }, config: {} }],
        layout: [{ id: "w_bill_" + ts, colSpan: 2, rowSpan: 1 }],
      });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  // Navigate to the custom page (fresh load → addInitScript applies the injection
  // before wasm boot, so the store hydrates with the bill account + page).
  await page.goto(BASE + "/p/" + PAGE_SLUG, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);

  // The widget body must show the bill name, not a "Pick a data source" error.
  const body = await page.locator(".wbody").first().textContent().catch(() => "");
  if (/pick a data source/i.test(body)) fail(`bills list widget shows "Pick a data source" — SourceBills not wired (L63 GAP-A). Body: ${body}`);
  if (!body.includes(BILL_NAME)) fail(`bill "${BILL_NAME}" not found in widget body. Body: "${body}"`);

  if (!process.exitCode) console.log(`PASS: Custom page bills list widget shows "${BILL_NAME}" (L63 GAP-A resolved).`);
} finally {
  await browser.close();
}
