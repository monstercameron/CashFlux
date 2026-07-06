import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
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
  await p.evaluate(()=>{history.pushState({},"","/health");window.dispatchEvent(new PopStateEvent("popstate"));});
  await p.waitForTimeout(1600);
  const r=await p.evaluate(()=>{
    const steps=[...document.querySelectorAll('[data-testid="health-step"]')];
    return {count:steps.length, first: steps[0]? steps[0].textContent.trim().slice(0,40):null, isButton: steps[0]?steps[0].tagName==='BUTTON':false};
  });
  console.log("  health steps: "+JSON.stringify(r));
  if(r.count>0 && r.isButton) pass("R52: "+r.count+" 'Where to focus next' steps are clickable drill buttons"); else { fail("no clickable health-step found (count="+r.count+")"); }
  if(r.count>0){
    await p.click('[data-testid="health-step"]'); await p.waitForTimeout(1200);
    const where=await p.evaluate(()=>location.pathname);
    console.log("  first step drilled to: "+where);
    const expected=["/transactions","/goals","/debt","/budgets","/credit","/accounts"];
    if(expected.includes(where)) pass("R52: clicking a focus step drills to its action screen ("+where+")"); else fail("unexpected drill target: "+where);
  }
  await p.screenshot({path:"e2e/screenshots/r52_health_steps.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
