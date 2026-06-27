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
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.evaluate(()=>{history.pushState({},"","/credit");window.dispatchEvent(new PopStateEvent("popstate"));});
  await p.waitForTimeout(1600);
  const r=await p.evaluate(()=>{
    const s=[...document.querySelectorAll('svg[role="img"]')].find(e=>/credit health score/i.test(e.getAttribute('aria-label')||""));
    if(!s) return {found:false, anyImg:[...document.querySelectorAll('svg[role="img"]')].map(e=>e.getAttribute('aria-label'))};
    const overlay=s.parentElement.querySelector('[aria-hidden="true"]');
    return {found:true, label:s.getAttribute('aria-label'), overlayHidden:!!overlay};
  });
  console.log("  "+JSON.stringify(r));
  if(r.found && /out of 100/.test(r.label)) pass("credit score ring has an accessible name: "+r.label); else fail("credit ring not labelled: "+JSON.stringify(r));
  if(r.overlayHidden) pass("overlay number is aria-hidden (no double announce)"); else fail("overlay not aria-hidden");
  await p.screenshot({path:"e2e/screenshots/credit_ring_a11y.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
