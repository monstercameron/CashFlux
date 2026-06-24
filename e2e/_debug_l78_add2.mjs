import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });
const errs = []; page.on("pageerror", e => errs.push(String(e)));
await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
const navTo = async (t) => { await page.evaluate((t)=>{const l=Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')).find(x=>x.getAttribute("title")===t); if(l)l.click();},t); await page.waitForTimeout(1500); };
await navTo("Transactions");
await page.waitForTimeout(800);

// counter text + set page size to All
const counter = await page.evaluate(()=> (document.body.textContent.match(/\b\d[\d,]*\s+(?:of\s+\d[\d,]*\s+)?transactions?\b/i)||[])[0]||null);
console.log("Counter text:", counter);

// open add
await page.evaluate(() => { const b=Array.from(document.querySelectorAll("button")).find(b=>/new transaction|add transaction|^\s*add\s*$/i.test(b.textContent.trim())); if(b)b.click(); });
await page.waitForTimeout(600);

// fill by REAL placeholders + click Expense button + pick Everyday Checking + a category
const fillR = await page.evaluate(() => {
  const log=[];
  const amt=document.querySelector('input[placeholder="Amount"]'); if(amt){amt.value="77.77";amt.dispatchEvent(new Event("input",{bubbles:true}));amt.dispatchEvent(new Event("change",{bubbles:true}));log.push("amt set");}
  const desc=document.querySelector('input[placeholder="What was it for?"]'); if(desc){desc.value="ZZDEBUG groceries";desc.dispatchEvent(new Event("input",{bubbles:true}));desc.dispatchEvent(new Event("change",{bubbles:true}));log.push("desc set");}
  const exp=Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==="Expense"); if(exp){exp.click();log.push("clicked Expense");}
  const acct=Array.from(document.querySelectorAll('select')).find(s=>s.getAttribute('aria-label')==="Account");
  if(acct){const o=Array.from(acct.options).find(o=>/Everyday Checking/i.test(o.text)); if(o){acct.value=o.value;acct.dispatchEvent(new Event("change",{bubbles:true}));log.push("acct=Checking");}}
  const cat=Array.from(document.querySelectorAll('select')).find(s=>s.getAttribute('aria-label')==="Category");
  if(cat){const o=Array.from(cat.options).find(o=>/Dining/i.test(o.text)); if(o){cat.value=o.value;cat.dispatchEvent(new Event("change",{bubbles:true}));log.push("cat=Dining");}}
  // is Save disabled?
  const save=Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==="Save");
  log.push("saveDisabled="+(save?save.disabled:"NO SAVE BTN"));
  return log;
});
console.log("FILL:", fillR);
await page.waitForTimeout(300);
await page.evaluate(()=>{const s=Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==="Save"); if(s)s.click();});
await page.waitForTimeout(1400);
const after = await page.evaluate(()=>({
  dialogOpen: !!document.querySelector('dialog[open],[role="dialog"]'),
  zz: (document.body.textContent.match(/ZZDEBUG/g)||[]).length,
  counter: (document.body.textContent.match(/\b\d[\d,]*\s+(?:of\s+\d[\d,]*\s+)?transactions?\b/i)||[])[0]||null,
}));
console.log("AFTER SAVE:", JSON.stringify(after));

// Now set page size to All and search again
await page.evaluate(()=>{ const sel=Array.from(document.querySelectorAll('select')).find(s=>Array.from(s.options).map(o=>o.text).join(",")==="25,50,100,All"); if(sel){sel.value=Array.from(sel.options).find(o=>o.text==="All").value; sel.dispatchEvent(new Event("change",{bubbles:true}));}});
await page.waitForTimeout(800);
const allView = await page.evaluate(()=>({
  zz:(document.body.textContent.match(/ZZDEBUG/g)||[]).length,
  counter:(document.body.textContent.match(/\b\d[\d,]*\s+(?:of\s+\d[\d,]*\s+)?transactions?\b/i)||[])[0]||null,
}));
console.log("ALL VIEW:", JSON.stringify(allView));
console.log("ERRS:", errs.slice(0,5));
await browser.close();
