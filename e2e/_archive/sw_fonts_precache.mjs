import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(5000);
  // wait for the SW to be registered + activated, then inspect the cache
  const info=await p.evaluate(async()=>{
    if(!('serviceWorker' in navigator)) return {sw:false};
    let reg=null;
    for(let i=0;i<30;i++){ reg=await navigator.serviceWorker.getRegistration(); if(reg && (reg.active)) break; await new Promise(r=>setTimeout(r,300)); }
    const keys=await caches.keys();
    const out={sw:!!reg, active:!!(reg&&reg.active), cacheNames:keys, hasFontsCss:false, coreCount:0};
    for(const k of keys){ const c=await caches.open(k); const reqs=await c.keys(); const urls=reqs.map(r=>new URL(r.url).pathname);
      if(urls.some(u=>/\/fonts\.css$/.test(u))) out.hasFontsCss=true; if(k.startsWith('cashflux-')) out.coreCount=urls.length; }
    return out;
  });
  console.log("  "+JSON.stringify(info));
  if(!info.sw){ fail("no service worker registered (env may not support it)"); }
  else {
    if(info.active) pass("service worker active"); else console.log("  (SW registered but not yet active)");
    if(info.hasFontsCss) pass("SW precache includes ./fonts.css (offline @font-face available)"); else fail("fonts.css NOT in any cache");
  }
  await browser.close();
}catch(e){fail("exception: "+e.message); await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
