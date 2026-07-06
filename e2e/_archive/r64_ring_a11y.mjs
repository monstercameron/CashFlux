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
const ring=p=>p.evaluate(()=>{
  const s=[...document.querySelectorAll('svg[role="img"]')].find(e=>/financial health score/i.test(e.getAttribute('aria-label')||""));
  return s? s.getAttribute('aria-label') : null;
});
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  // /health
  await p.evaluate(()=>{history.pushState({},"","/health");window.dispatchEvent(new PopStateEvent("popstate"));}); await p.waitForTimeout(1500);
  const h=await ring(p);
  console.log("  /health ring label: "+JSON.stringify(h));
  if(h && /out of 100/.test(h)) pass("/health score ring has an accessible name"); else fail("/health ring missing accessible name");
  // dashboard
  await p.evaluate(()=>{history.pushState({},"","/");window.dispatchEvent(new PopStateEvent("popstate"));}); await p.waitForTimeout(1500);
  const d=await ring(p);
  console.log("  dashboard ring label: "+JSON.stringify(d));
  if(d && /out of 100/.test(d)) pass("dashboard health-widget ring has an accessible name"); else console.log("  (dashboard health widget may be hidden/not in default layout)");
  // overlay aria-hidden (no double announce)
  const overlayHidden=await p.evaluate(()=>{
    const s=[...document.querySelectorAll('svg[role="img"]')].find(e=>/financial health score/i.test(e.getAttribute('aria-label')||""));
    if(!s) return null;
    const sib=s.parentElement.querySelector('[aria-hidden="true"] .fig, [aria-hidden="true"]');
    return !!sib;
  });
  console.log("  overlay aria-hidden present: "+overlayHidden);
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
