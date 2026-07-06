// L-quickhit #43 gate — a saved formula loaded into the editor and re-saved must
// UPDATE in place, not create a duplicate. Previously every Save minted a new ID,
// so loading then saving silently duplicated the formula. Loads the seeded
// formulas, edits one's name, saves, and asserts the formula count is unchanged.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
try {
  const p = await browser.newPage();
  p.on("pageerror", (e) => fail("page error: " + e.message));
  await p.goto(BASE + "/customize", { waitUntil: "domcontentloaded" });
  await p.waitForTimeout(1500);
  const flush = async () => { await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await p.waitForTimeout(400); };
  const count = async () => { await flush(); return p.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").formulas?.length || 0); };
  const before = await count();
  if (before === 0) { fail("no seeded formulas to test"); process.exit(1); }
  const editBtns = p.locator('button:has-text("Edit")');
  if ((await editBtns.count()) === 0) { fail("no Edit (load) button on a saved formula"); process.exit(1); }
  await editBtns.first().click();
  await p.waitForTimeout(400);
  await p.locator('input[type=text]').first().fill("Edited by gate " + before);
  await p.locator('button:has-text("Save")').first().click();
  await p.waitForTimeout(500);
  const after = await count();
  if (after !== before) fail(`formula count changed ${before} -> ${after}; load+save should update in place`);
  if (!process.exitCode) console.log(`PASS: load+edit+save updates in place (formulas stayed at ${after}).`);
} finally {
  await browser.close();
}
