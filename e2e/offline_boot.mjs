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
  p.on("pageerror",e=>errs.push(String(e)));
  // 1) Online load — SW installs + precaches core (incl. wasm) + runtime-caches assets.
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(5000);
  // wait for SW active + controlling
  await p.evaluate(async()=>{ for(let i=0;i<40;i++){ const r=await navigator.serviceWorker.getRegistration(); if(r&&r.active&&navigator.serviceWorker.controller) return; await new Promise(x=>setTimeout(x,300)); } });
  // confirm wasm is in the cache (C312 probe)
  const cacheState=await p.evaluate(async()=>{
    const out={wasm:false, indexHtml:false, fontsCss:false, controller:!!navigator.serviceWorker.controller};
    for(const k of await caches.keys()){ const c=await caches.open(k); for(const req of await c.keys()){ const u=new URL(req.url).pathname;
      if(/main\.wasm$/.test(u)) out.wasm=true; if(/index\.html$|\/$/.test(u)) out.indexHtml=true; if(/fonts\.css$/.test(u)) out.fontsCss=true; } }
    return out;
  });
  console.log("  cache after online load: "+JSON.stringify(cacheState));
  if(cacheState.wasm) pass("C312: main.wasm IS cached by the SW (offline boot possible)"); else fail("C312 CONFIRMED: main.wasm NOT cached → offline would be blank");
  if(!cacheState.controller) console.log("  (SW not yet controlling — reload to gain control)");
  // reload once to ensure SW controls
  await p.reload({waitUntil:"networkidle"}); await p.waitForTimeout(3000);
  // 2) Go OFFLINE and reload.
  await ctx.setOffline(true);
  console.log("  -> offline; reloading");
  let booted=false, bodyText="";
  try{ await p.reload({waitUntil:"domcontentloaded", timeout:30000}); }catch(e){ console.log("  reload note:",e.message.slice(0,60)); }
  await p.waitForTimeout(6000);
  const r=await p.evaluate(()=>({ hasApp:!!document.querySelector('#app'), appLen:(document.querySelector('#app')?.innerText||"").trim().length, title:document.title, bodyLen:document.body.innerText.trim().length }));
  console.log("  offline reload: "+JSON.stringify(r));
  if(r.hasApp && r.appLen>30) pass("C311: app BOOTS offline (not a blank page) — #app rendered "+r.appLen+" chars"); else fail("C311: offline reload BLANK (#app empty, len="+r.appLen+")");
  await p.screenshot({path:"e2e/screenshots/offline_boot.png"});
  await ctx.setOffline(false);
  console.log("pageerrors:", errs.length); errs.slice(0,4).forEach(e=>console.log("  ERR:"+e.slice(0,100)));
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
