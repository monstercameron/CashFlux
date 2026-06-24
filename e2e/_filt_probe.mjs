import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const b = await chromium.launch({headless:true}); const p = await b.newPage(); p.setViewportSize({width:1280,height:1000});
await p.goto("http://127.0.0.1:8099/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Transactions");if(l)l.click();});
await p.waitForTimeout(1500);
const before=await p.evaluate(()=>{
  const summary=document.body.textContent.match(/[\d,]+\s+transactions?\s+shown[^\n]{0,40}/i);
  const counter=document.body.textContent.match(/([\d,]+)\s+transactions?/i);
  const filtersBtn=[...document.querySelectorAll('button')].find(b=>/^filters$/i.test(b.textContent.trim()));
  const catSel=document.querySelector('select[aria-label="Filter by category"]');
  return {summary:summary?summary[0]:null, counter:counter?counter[0]:null, hasFiltersBtn:!!filtersBtn, catSelVisible:catSel?catSel.offsetParent!==null:false};
});
console.log("BEFORE:", JSON.stringify(before));
// open Filters if collapsed
await p.evaluate(()=>{const b=[...document.querySelectorAll('button')].find(b=>/^filters$/i.test(b.textContent.trim()));if(b)b.click();});
await p.waitForTimeout(600);
const opened=await p.evaluate(()=>{
  const cat=document.querySelector('select[aria-label="Filter by category"]');
  const acct=document.querySelector('select[aria-label="Filter by account"]');
  const search=document.querySelector('input[placeholder="Search description or tag"]');
  return {catVisible:cat?cat.offsetParent!==null:false, catOpts:cat?[...cat.options].map(o=>o.text).slice(0,8):null, acctVisible:!!acct, searchVisible:!!search};
});
console.log("OPENED:", JSON.stringify(opened,null,1));
await b.close();
