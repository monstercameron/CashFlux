// L108 E2E loop story — "Closing an Account" (Marcus) — 2026-06-25
//
// Theme: ACCOUNT ARCHIVE integrity across Accounts <-> Dashboard. Archiving a closed account must
// remove it from the ACTIVE list and EXCLUDE it from net worth by EXACTLY its balance (ledger excludes
// archived accounts) — without deleting it.
//
// Invariants:
//   A-1  An asset account shows its balance; net worth is readable.
//   A-2  Archiving it (via the row's overflow menu) removes it from the ACTIVE accounts list.
//   A-3  Net worth DECREASES by EXACTLY that account's balance (archived excluded).
//   A-4  No JS errors.
//
// Run: node e2e/loopstory_108_closing_an_account.mjs
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass = (l) => { console.log("PASS:   " + l); passed++; };
const fail = (l) => { console.error("FAIL:   " + l); failed++; };
const absent_ = (l) => { console.log("ABSENT: " + l); absent++; };
const note = (l) => { console.log("NOTE:   " + l); };
const navTo = async (page, title) => { await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title); await page.waitForTimeout(1200); };
const readNetWorth = (page) => page.evaluate(() => { const els = [...document.querySelectorAll('*')].filter(e => e.children.length <= 3 && /net worth/i.test(e.textContent || "") && (e.textContent || "").length < 80); for (const e of els) { const m = (e.textContent || "").match(/net worth[^$(-]*\(?-?\$([\d,]+\.?\d*)\)?/i); if (m) return parseFloat(m[1].replace(/,/g, "")); } return null; });
// active account row by name: read its last $ as balance + whether it's in the active (non-archived) list
const readAcct = (page, name) => page.evaluate((name) => {
  const rows = [...document.querySelectorAll('.row')];
  const r = rows.find(x => (x.textContent || "").includes(name) && !(x.closest('[class*="archiv"]')));
  if (!r) return null;
  const ds = [...(r.textContent || "").matchAll(/\$([\d,]+\.?\d*)/g)].map(m => parseFloat(m[1].replace(/,/g, "")));
  return { bal: ds.length ? ds[ds.length - 1] : null };
}, name);
// An archived account moves to the "Archived" section and its row offers "Restore" (active rows show
// "Archive"/"Update balance", never "Restore"). So "archived" = a row with the name containing "Restore".
const isArchived = (page, name) => page.evaluate((name) => [...document.querySelectorAll('.row')].some(r => (r.textContent || "").includes(name) && /restore/i.test(r.textContent || "")), name);
const jsErrors = [];
const TARGET = "Roth IRA";
try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);
  await navTo(page, "Dashboard");
  const nw0 = await readNetWorth(page);
  await navTo(page, "Accounts");
  const a0 = await readAcct(page, TARGET);
  note("Net worth before: $" + nw0 + " · " + TARGET + " balance: $" + (a0 && a0.bal));
  if (nw0 != null && a0 && a0.bal != null) pass("A-1 — net worth $" + nw0 + ", " + TARGET + " $" + a0.bal);
  else { absent_("A-1 — baseline unreadable (nw=" + nw0 + ", acct=" + JSON.stringify(a0) + ")"); throw new Error("baseline"); }
  // archive via the row's overflow menu -> "Archive"
  const arch = await page.evaluate((name) => {
    const r = [...document.querySelectorAll('.row')].find(x => (x.textContent || "").includes(name) && x.querySelector('button[aria-label="More actions"]'));
    if (!r) return "NO_ROW";
    const m = r.querySelector('button[aria-label="More actions"]'); m.click();
    return "menu-open";
  }, TARGET);
  await page.waitForTimeout(450);
  const clicked = await page.evaluate(() => { const item = [...document.querySelectorAll('.add-item, button')].find(b => b.offsetParent !== null && /^archive$/i.test((b.textContent || "").trim())); if (item) { item.click(); return "archived"; } return "NO_ARCHIVE_ITEM"; });
  await page.waitForTimeout(1100);
  note("archive: " + arch + " / " + clicked);
  const archived1 = await isArchived(page, TARGET);
  await page.screenshot({ path: path.join(SSDIR, "L108_archived.png") });
  if (clicked === "archived" && archived1) pass("A-2 — " + TARGET + " moved to the Archived section (offers Restore) — out of the active list");
  else fail("A-2 — " + TARGET + " not archived (clicked=" + clicked + ", archived=" + archived1 + ")");
  await navTo(page, "Dashboard");
  const nw1 = await readNetWorth(page);
  const drop = (nw0 || 0) - (nw1 || 0);
  note("Net worth after: $" + nw1 + " (drop $" + drop.toFixed(2) + ", expected $" + (a0 && a0.bal) + ")");
  if (nw1 != null && Math.abs(drop - a0.bal) <= 0.01) pass("A-3 — net worth dropped by EXACTLY the archived balance ($" + nw0 + " -> $" + nw1 + ", -$" + a0.bal + ")");
  else fail("A-3 — net worth drop $" + drop.toFixed(2) + " != archived balance $" + (a0 && a0.bal) + " ($" + nw0 + " -> $" + nw1 + ")");
  if (jsErrors.length === 0) pass("A-4 — zero runtime JS errors across the ritual");
  else fail("A-4 — " + jsErrors.length + " JS errors: " + jsErrors.slice(0, 3).join("; "));
} catch (err) {
  if (String(err.message) !== "baseline") { fail("UNEXPECTED_ERROR — " + err.message); console.error(err); }
} finally { await browser.close(); }
console.log("\n════════════════════════════════════════════");
console.log("RESULT: " + passed + " PASS · " + failed + " FAIL · " + absent + " ABSENT");
console.log("════════════════════════════════════════════");
process.exit(failed > 0 ? 1 : 0);
