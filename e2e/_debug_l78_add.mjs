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

// 1. Dump the txn list structure & any pagination / count text
const info = await page.evaluate(() => {
  const out = {};
  out.bodyHasCount = (document.body.textContent.match(/\b\d+\s+transactions?\b/i) || [])[0] || null;
  out.tbodyRows = document.querySelectorAll('tbody tr').length;
  out.dataCfRows = document.querySelectorAll('[data-cf="txn-row"]').length;
  // pagination controls?
  out.pager = Array.from(document.querySelectorAll('button,a')).map(b=>b.textContent.trim()).filter(t=>/next|prev|page|show more|load more|1.*2.*3|of \d/i.test(t)).slice(0,10);
  // sort control
  out.selects = Array.from(document.querySelectorAll('select')).map(s=>({label:s.getAttribute('aria-label'),opts:Array.from(s.options).map(o=>o.text).slice(0,6)}));
  // first & last visible row text
  const rows = document.querySelectorAll('tbody tr');
  out.firstRow = rows[0]?.textContent.trim().slice(0,80);
  out.lastRow = rows[rows.length-1]?.textContent.trim().slice(0,80);
  return out;
});
console.log("LIST INFO:", JSON.stringify(info, null, 2));

// 2. Open add form, dump every field present
await page.evaluate(() => { const b=Array.from(document.querySelectorAll("button")).find(b=>/new transaction|add transaction|^\s*add\s*$/i.test(b.textContent.trim())); if(b)b.click(); });
await page.waitForTimeout(600);
const form = await page.evaluate(() => {
  const inputs = Array.from(document.querySelectorAll('input,textarea,select')).map(i=>({
    tag:i.tagName, type:i.type, label:i.getAttribute('aria-label'), ph:i.getAttribute('placeholder'),
    required:i.required, value:i.value,
    opts: i.tagName==='SELECT'?Array.from(i.options).map(o=>o.text).slice(0,8):undefined
  }));
  const buttons = Array.from(document.querySelectorAll('dialog button, [role="dialog"] button, form button')).map(b=>b.textContent.trim());
  return { inputs, buttons };
});
console.log("\nADD FORM:", JSON.stringify(form, null, 2));

// 3. Actually fill & submit one, watch for validation errors
await page.evaluate(() => {
  const d=Array.from(document.querySelectorAll("input,textarea")).find(i=>/description|payee|note/i.test(i.getAttribute("aria-label")||i.getAttribute("placeholder")||""));
  if(d){d.value="ZZDEBUG one";d.dispatchEvent(new Event("input",{bubbles:true}));d.dispatchEvent(new Event("change",{bubbles:true}));}
  const n=document.querySelector('input[type="number"]'); if(n){n.value="99.99";n.dispatchEvent(new Event("input",{bubbles:true}));n.dispatchEvent(new Event("change",{bubbles:true}));}
});
await page.waitForTimeout(300);
const beforeSubmit = await page.evaluate(()=>document.querySelectorAll('tbody tr').length);
await page.evaluate(() => { const b=Array.from(document.querySelectorAll("button")).find(b=>/^add$|^save$|^add transaction$/i.test(b.textContent.trim())&&b.type!=="reset"); if(b)b.click(); });
await page.waitForTimeout(1200);
const after = await page.evaluate(() => {
  return {
    rows: document.querySelectorAll('tbody tr').length,
    dialogStillOpen: !!document.querySelector('dialog[open],[role="dialog"]'),
    validationMsgs: Array.from(document.querySelectorAll('[role="alert"],.error,.invalid,[aria-invalid="true"]')).map(e=>e.textContent.trim()).slice(0,5),
    zzVisible: (document.body.textContent.match(/ZZDEBUG/g)||[]).length,
  };
});
console.log(`\nSUBMIT: rowsBefore=${beforeSubmit} rowsAfter=${after.rows} dialogOpen=${after.dialogStillOpen} zzVisible=${after.zzVisible} validation=${JSON.stringify(after.validationMsgs)}`);
console.log("JS ERRORS:", errs.slice(0,5));
await browser.close();
