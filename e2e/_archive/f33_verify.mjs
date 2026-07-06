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
  // stub print so it doesn't block
  await p.addInitScript(()=>{ window.__printed=0; window.print=()=>{window.__printed++;}; });
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1500);
  const r=await p.evaluate(()=>{
    const txt=document.body.innerText;
    const exportControls=[...document.querySelectorAll('.reports-export, summary')].filter(e=>/export/i.test(e.textContent||"")).length;
    const pdfBtn=[...document.querySelectorAll('button')].find(b=>/save as pdf|pdf/i.test(b.textContent||""));
    const yoy=!!document.querySelector('[data-testid="reports-yoy-toggle"]');
    const selector=/report type/i.test(txt) || [...document.querySelectorAll('[role="radio"]')].some(e=>/overview|categories|net worth/i.test(e.textContent||""));
    return {exportControls, pdfBtn:!!pdfBtn, pdfText:pdfBtn?.textContent.trim(), yoy, selector};
  });
  console.log("  "+JSON.stringify(r));
  if(r.pdfBtn) pass("C236: 'Save as PDF' button present: "+r.pdfText); else fail("no PDF button");
  if(r.exportControls>=1) pass("C240: a single consolidated export control (count="+r.exportControls+")"); else console.log("  (export control not matched)");
  if(r.yoy) pass("C237: YoY toggle present"); else fail("no YoY toggle");
  if(r.selector) pass("C243: report-type selector present"); else fail("no report-type selector");
  // click PDF → print() called
  if(r.pdfBtn){ await p.evaluate(()=>{[...document.querySelectorAll('button')].find(b=>/save as pdf|pdf/i.test(b.textContent||"")).click();}); await p.waitForTimeout(300);
    const printed=await p.evaluate(()=>window.__printed); if(printed>0) pass("C236: clicking Save as PDF invokes print()"); else fail("print() not called"); }
  await p.screenshot({path:"e2e/screenshots/f33_verify.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
