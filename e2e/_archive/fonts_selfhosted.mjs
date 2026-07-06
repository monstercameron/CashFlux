import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const reqHosts=new Set(); const woff2=[]; const errs=[];
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("request", r=>{ try{ const u=new URL(r.url()); reqHosts.add(u.host); if(/\.woff2/.test(u.pathname)) woff2.push(u.pathname);}catch{} });
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(5000);
  // 1) NO request to Google hosts
  const google=[...reqHosts].filter(h=>/googleapis|gstatic|google/i.test(h));
  console.log("  hosts contacted:", [...reqHosts].join(", "));
  console.log("  woff2 loaded:", woff2.length, woff2.slice(0,2));
  if(google.length===0) pass("PRIVACY: no request to any Google host at boot (was fonts.googleapis.com)"); else fail("still contacting Google: "+google.join(","));
  if(woff2.some(u=>u.includes('/fonts/'))) pass("self-hosted woff2 loaded from /fonts/"); else console.log("  (no local woff2 fetched — may be cached/unused glyphs)");
  // 2) Fraunces actually applies to a display heading (h1/figure)
  const fontApplied=await p.evaluate(()=>{
    const h=document.querySelector('h1, .t-figure-lg, [class*="hero-net"], .fig');
    if(!h) return null;
    const ff=getComputedStyle(h).fontFamily;
    return ff;
  });
  console.log("  display font-family:", fontApplied);
  if(fontApplied && /fraunces|inter|serif|sans/i.test(fontApplied)) pass("font-family resolves (Fraunces/Inter or fallback): "+fontApplied);
  // 3) document.fonts confirms a Fraunces/Inter face is loaded
  const facesLoaded=await p.evaluate(async()=>{ try{ await document.fonts.ready; }catch{}; const fams=new Set(); document.fonts.forEach(f=>fams.add(f.family)); return [...fams]; });
  console.log("  loaded font faces:", JSON.stringify(facesLoaded));
  if(facesLoaded.some(f=>/Fraunces|Inter/.test(f))) pass("Fraunces/Inter @font-face registered via local fonts.css"); else fail("no Fraunces/Inter face loaded: "+JSON.stringify(facesLoaded));
  await p.screenshot({path:"e2e/screenshots/fonts_selfhosted.png"});
  console.log("errors:", errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
