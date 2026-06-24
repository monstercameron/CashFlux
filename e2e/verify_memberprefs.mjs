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
  // Open the first member's inline editor.
  const editBtn = await page.$('button:has-text("Edit")');
  log(!!editBtn, "member Edit button present");
  await editBtn.click();
  await page.waitForTimeout(400);
  const form = await page.$(".row form.form-grid");
  log(!!form, "inline member editor opened");
  // The editor must carry the two per-member preference selects.
  const labels = await page.$$eval(".row form.form-grid label, .row form.form-grid .field-label", els => els.map(e=>e.textContent.trim()));
  const allText = await page.$eval(".row form.form-grid", e => e.textContent);
  log(/date style/i.test(allText), "per-member 'Date style' field present");
  log(/default account/i.test(allText), "per-member 'Default account' field present");
  const selects = form ? await form.$$("select") : [];
  log(selects.length >= 2, `editor has ≥2 selects (got ${selects.length})`);
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
