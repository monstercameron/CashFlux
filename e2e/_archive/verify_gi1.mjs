import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
let pass = true; const log = (c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  await page.goto(base + "/rules", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  // The inline add-rule form must be present WITHOUT clicking the +Add button.
  const form = await page.$('[data-testid="rule-add-form"]');
  log(!!form, "inline rule-add-form present on /rules without opening a modal");
  const heading = await page.$('text=/quick add a rule/i');
  log(!!heading, "'Quick add a rule' heading present");
  // It must carry the match input + a category select + an Add button.
  const matchInput = form && await form.$('#rule-add');
  log(!!matchInput, "match input present in inline form");
  const addBtn = form && await form.$('button[type="submit"]');
  log(!!addBtn, "Add button present in inline form");
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
