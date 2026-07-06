import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
let pass = true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  await page.goto(base + "/accounts", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  const s = await page.$("text=/load sample/i"); if (s) { await s.click().catch(()=>{}); await page.waitForTimeout(700); }
  await page.goto(base + "/members", { waitUntil: "networkidle" });
  await page.waitForSelector(".row", { timeout: 10000 });
  await (await page.$('button:has-text("Edit")')).click();
  await page.waitForTimeout(300);
  // Select a non-default date style (US) on the first select and save.
  const sel = (await page.$$(".row form.form-grid select"))[0];
  await sel.selectOption("us");
  await (await page.$('.row form.form-grid button[type="submit"]')).click();
  await page.waitForTimeout(500);
  // Reopen and confirm the value persisted.
  await (await page.$('button:has-text("Edit")')).click();
  await page.waitForTimeout(300);
  const val = await page.$eval(".row form.form-grid select", e => e.value);
  log(val === "us", `per-member date style persisted across save/reopen (got "${val}")`);
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
