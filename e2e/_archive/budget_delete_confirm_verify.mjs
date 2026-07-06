import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const b = await chromium.launch({headless:true}); let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const ctx=await b.newContext(); const p=await ctx.newPage(); p.setViewportSize({width:1440,height:1000});
await p.goto("http://127.0.0.1:8099/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
await p.waitForTimeout(1400);
await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Budgets"); if(l)l.click();});
await p.waitForTimeout(1100);
const before=await p.evaluate(()=>document.querySelectorAll('.budget').length);
// click delete
await p.evaluate(()=>{const btns=[...document.querySelectorAll('button')].filter(b=>/delete budget/i.test(b.getAttribute('aria-label')||'')); if(btns[0])btns[0].click();});
await p.waitForTimeout(700);
// 1) a confirm dialog should appear (not deleted yet)
const dlg=await p.evaluate(()=>{const d=document.querySelector('.cf-dialog'); return d?{msg:d.textContent.trim().slice(0,80),hasCancel:!!d.querySelector('#cf-dialog-cancel'),hasConfirm:!!d.querySelector('#cf-dialog-confirm')}:null;});
const midCount=await p.evaluate(()=>document.querySelectorAll('.budget').length);
if(dlg) P(`confirm dialog appears on delete: "${dlg.msg}" (cancel=${dlg.hasCancel}, confirm=${dlg.hasConfirm})`); else F("no confirm dialog on delete");
if(midCount===before) P(`budget NOT deleted before confirming (${midCount})`); else F(`budget deleted before confirm (${before}->${midCount})`);
// 2) cancel -> budget restored/unchanged
await p.evaluate(()=>{const c=document.querySelector('#cf-dialog-cancel'); if(c)c.click();});
await p.waitForTimeout(600);
const afterCancel=await p.evaluate(()=>document.querySelectorAll('.budget').length);
if(afterCancel===before) P(`cancel keeps the budget (${afterCancel})`); else F(`cancel changed count (${before}->${afterCancel})`);
// 3) delete + confirm -> deleted
await p.evaluate(()=>{const btns=[...document.querySelectorAll('button')].filter(b=>/delete budget/i.test(b.getAttribute('aria-label')||'')); if(btns[0])btns[0].click();});
await p.waitForTimeout(600);
await p.evaluate(()=>{const c=document.querySelector('#cf-dialog-confirm'); if(c)c.click();});
await p.waitForTimeout(800);
const afterConfirm=await p.evaluate(()=>document.querySelectorAll('.budget').length);
if(afterConfirm===before-1) P(`confirming deletes exactly one (${before}->${afterConfirm})`); else F(`confirm delete count wrong (${before}->${afterConfirm})`);
await p.screenshot({path:'e2e/screenshots/budget_delete_confirm.png'});
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
