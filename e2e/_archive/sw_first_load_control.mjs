import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
try{
  // Fresh context = no pre-existing SW. C313: does skipWaiting+clients.claim give the
  // SW control on the FIRST load, with NO reload?
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded"});
  await p.waitForSelector("#app",{timeout:60000});
  // Poll for controller WITHOUT reloading. Record how long it takes.
  const res=await p.evaluate(async()=>{
    const t0=Date.now();
    for(let i=0;i<60;i++){
      if(navigator.serviceWorker.controller) return {controlled:true, ms:Date.now()-t0};
      await new Promise(x=>setTimeout(x,200));
    }
    const r=await navigator.serviceWorker.getRegistration();
    return {controlled:false, ms:Date.now()-t0, hasReg:!!r, active:!!(r&&r.active)};
  });
  console.log("  first-load control: "+JSON.stringify(res));
  if(res.controlled) pass("C313: SW controls the page on FIRST load ("+res.ms+"ms, no reload needed)");
  else fail("C313 CONFIRMED: SW did NOT take control on first load (reg="+res.hasReg+" active="+res.active+") — needs a 2nd load");
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
