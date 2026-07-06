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
  // Smart is OFF by default — confirm anomalies still surface (ungated).
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.click('a[href="/insights"]'); await p.waitForTimeout(1700);
  const r=await p.evaluate(()=>{
    const txt=document.body.innerText;
    // flagged-activity anomaly rows (SmartAnomalyInsightRow) — look for a flagged section + rows
    const rows=document.querySelectorAll('[data-testid^="anomaly"], [class*="anomaly"], [data-testid="smart-anomaly-row"]').length;
    const flaggedSection=/flagged|unusual|anomal|needs a look|heads.?up|duplicate|missing|spike/i.test(txt);
    return {flaggedSection, rows, len: txt.length};
  });
  console.log("  "+JSON.stringify(r));
  if(r.flaggedSection) pass("C252: anomaly/flagged-activity content present on /insights (Smart off by default → ungated)"); else fail("no anomaly section on /insights");
  await p.screenshot({path:"e2e/screenshots/c252_anomalies.png", fullPage:true});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
