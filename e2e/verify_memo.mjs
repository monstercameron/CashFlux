import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
let pass = true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
const readSpend = async (page) => { await page.goto(base + "/dashboard", { waitUntil: "networkidle" }); await page.waitForSelector(".bento",{timeout:15000}); await page.waitForTimeout(700);
  return page.evaluate(() => { const t=[...document.querySelectorAll(".w")].find(x=>/spending|expense/i.test(x.innerText)); return t? t.innerText.replace(/\s+/g,' ') : null; }); };
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  await page.goto(base + "/accounts", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  const s = await page.$("text=/load sample/i"); if (s) { await s.click().catch(()=>{}); await page.waitForTimeout(800); }
  const before = await readSpend(page);
  log(!!before, `spending KPI (memoized): ${before}`);
  // Open quick-add, fill amount, click the panel's Save button (reliable submit).
  await page.keyboard.press("Alt+KeyN");
  await page.waitForSelector('.flip-backdrop.show, input[type="number"]', { timeout: 5000 });
  await page.waitForTimeout(300);
  await page.fill('input[type="number"]', "777.77");
  await page.fill('.flip-backdrop input[type="text"], input[type="text"]', "Memo test expense");
  // Click the primary Save/Add button inside the flip panel.
  const saveBtn = await page.$('.set-btn.save');
  log(!!saveBtn, "quick-add Save button found");
  await saveBtn.click();
  await page.waitForTimeout(1000);
  const after = await readSpend(page);
  log(before !== after, `memoized spending recomputed after add (NOT stale): "${before}" -> "${after}"`);
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
