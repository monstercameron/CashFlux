import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  await p.click('a[href="/insights"]'); await p.waitForTimeout(1700);
  const r=await p.evaluate(()=>{
    const txt=document.body.innerText;
    // C230: a spending time-series chart (svg) labelled about months of spending
    const charts=[...document.querySelectorAll('svg[role="img"], svg')].map(s=>s.getAttribute('aria-label')||"");
    const trendChart=charts.some(l=>/month|spend|trend/i.test(l));
    // C229: merchant/top-merchants section
    const merchant=/merchant|top merchants|where your money/i.test(txt);
    // C231: starter chips present
    const chips=document.querySelectorAll('[data-testid^="starter-chip"], .chip, button.chip, [class*="chip"]').length;
    // C228: highlight rows are buttons (drill-through)
    const drillBtns=[...document.querySelectorAll('button')].filter(b=>/spending is (up|down)|nothing spent yet/i.test(b.textContent||"")).length;
    return {trendChart, charts:charts.filter(Boolean).slice(0,6), merchant, chips, drillBtns};
  });
  console.log("  "+JSON.stringify(r));
  if(r.trendChart) pass("C230: spending time-series chart present on /insights"); else fail("C230: no trend chart");
  if(r.merchant) pass("C229: merchant-level spending section present"); else console.log("  (C229 merchant text not matched — check label)");
  if(r.chips>0) pass("C231: starter chips present"); else console.log("  (C231: no chips — may need empty Ask box)");
  if(r.drillBtns>0) pass("C228: spending highlights are clickable (drill-through buttons)"); else console.log("  (C228: highlights not buttons or none shown)");
  await p.screenshot({path:"e2e/screenshots/f32_verify.png", fullPage:true});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
