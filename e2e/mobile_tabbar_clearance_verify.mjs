import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
// MOBILE: last control clears the fixed tab bar
const mp=await browser.newPage(); mp.setViewportSize({width:390,height:840});
await mp.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await mp.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await mp.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Transactions");if(l)l.click();});
await mp.waitForTimeout(1300);
await mp.evaluate(()=>{const m=document.querySelector('main.cf-scroll');m.scrollTop=m.scrollHeight;}); await mp.waitForTimeout(600);
const r=await mp.evaluate(()=>{
  const sc=document.querySelector('main.cf-scroll'); const pb=getComputedStyle(sc).paddingBottom;
  const tab=document.querySelector('.mobile-tabbar'); const tabTop=tab.getBoundingClientRect().top;
  // the "Rows per page" / page-size buttons — are they above the tab bar now?
  const pageSize=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==="All" && /25|50|100/.test(b.parentElement.textContent));
  const psBottom=pageSize?pageSize.getBoundingClientRect().bottom:null;
  return {paddingBottom:pb, tabTop:Math.round(tabTop), pageSizeBottom:psBottom?Math.round(psBottom):null};
});
console.log("MOBILE:", JSON.stringify(r));
if(parseFloat(r.paddingBottom)>=56) P(`mobile scroller has bottom clearance (${r.paddingBottom})`); else F(`no clearance: ${r.paddingBottom}`);
if(r.pageSizeBottom!==null && r.pageSizeBottom <= r.tabTop) P(`page-size selector (bottom ${r.pageSizeBottom}) clears the tab bar (top ${r.tabTop})`); else F(`page-size still under tab bar: ${r.pageSizeBottom} vs ${r.tabTop}`);
await mp.screenshot({path:'e2e/screenshots/mobile_tabbar_fixed.png'}); await mp.close();
// DESKTOP: padding NOT applied (no regression)
const dp=await browser.newPage(); dp.setViewportSize({width:1280,height:900});
await dp.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000}); await dp.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
const dpb=await dp.evaluate(()=>getComputedStyle(document.querySelector('main.cf-scroll')).paddingBottom);
console.log("DESKTOP scroller paddingBottom:", dpb);
if(parseFloat(dpb)<56) P(`desktop scroller unaffected (${dpb})`); else F(`desktop got mobile padding: ${dpb}`);
await dp.close(); await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
