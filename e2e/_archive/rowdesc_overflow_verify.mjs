import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const p = await b.newPage(); p.setViewportSize({width:1280,height:1000});
await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
// (1) generic list-row .row-desc no longer overflows with a long no-space name
await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Goals");if(l)l.click();});
await p.waitForTimeout(1200);
const g=await p.evaluate(()=>{
  const cell=[...document.querySelectorAll('.row-desc')].find(e=>!e.closest('.txn-table')); if(!cell)return null;
  cell.textContent="X"+"Supercalifragilisticexpialidocious".repeat(5);
  const cs=getComputedStyle(cell); const rc=cell.getBoundingClientRect(); const pr=cell.parentElement.getBoundingClientRect();
  return {overflowWrap:cs.overflowWrap, cellRight:Math.round(rc.right), parentRight:Math.round(pr.right), overflowsParent:rc.right>pr.right+2, docOverflow:document.documentElement.scrollWidth-document.documentElement.clientWidth};
});
console.log("Goals list-row:", JSON.stringify(g));
if(g && g.overflowWrap==="anywhere" && !g.overflowsParent && g.docOverflow===0) P("list-row .row-desc no longer overflows with a long no-space name (wraps inside the card)");
else F("list-row still overflows: "+JSON.stringify(g));
await p.screenshot({path:'e2e/screenshots/rowdesc_nospace_after.png'});
// (2) txn-table row-desc still truncates (nowrap+ellipsis), not affected
await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Transactions");if(l)l.click();});
await p.waitForTimeout(1200);
const t=await p.evaluate(()=>{const c=[...document.querySelectorAll('.txn-table td.row-desc')][0]; if(!c)return null; const cs=getComputedStyle(c); return {whiteSpace:cs.whiteSpace, textOverflow:cs.textOverflow, maxW:cs.maxWidth};});
console.log("Txn-table row-desc:", JSON.stringify(t));
if(t && t.whiteSpace==="nowrap" && t.textOverflow==="ellipsis" && t.maxW==="280px") P("txn-table row-desc truncation unaffected (nowrap+ellipsis+max-width)");
else F("txn-table row-desc changed: "+JSON.stringify(t));
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
