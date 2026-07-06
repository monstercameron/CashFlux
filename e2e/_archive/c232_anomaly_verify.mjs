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
  await p.click('a[href="/insights"]'); await p.waitForTimeout(1600);
  const r=await p.evaluate(()=>{
    const txt=document.body.innerText;
    // count highlight sentences by direction phrasing
    const downCount=(txt.match(/spending is down \d+%/g)||[]).length;
    const upCount=(txt.match(/spending is up \d+%/g)||[]).length;
    // any literal "down 100%" false positive?
    const down100=/down 100%/.test(txt);
    return {downCount, upCount, down100, hasHighlights: upCount+downCount>0 || /nothing spent yet this month/.test(txt)};
  });
  console.log("  highlights: "+JSON.stringify(r));
  // Today (~June 27) the month is <90% elapsed → SuppressDecrease active → no "down N%" highlights.
  if(r.downCount===0) pass("no 'spending is down N%' highlights mid-month (decreases suppressed, C232)"); else fail("found "+r.downCount+" down-highlights mid-month (should be suppressed)");
  if(!r.down100) pass("no literal 'down 100%' false positive present"); else fail("'down 100%' still present");
  console.log("  (up-direction highlights still allowed: "+r.upCount+")");
  await p.screenshot({path:"e2e/screenshots/c232_anomaly.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
