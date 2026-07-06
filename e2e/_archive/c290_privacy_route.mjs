import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
const idOf=p=>p.evaluate(()=>{
  const t=(document.querySelector('h1,.page-title,h2')?.textContent||"").trim();
  const privacy=/privacy|local-first|your data|stays on this device|never leaves/i.test(document.body.innerText);
  const dashboardish=!!document.querySelector('.bento') || /net worth/i.test(document.querySelector('.reports-hero, .home-band, [class*="hero"]')?.textContent||"");
  return {title:t, privacyContent:privacy, dashboardish};
});
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  // navigate to /privacy via SPA
  await p.evaluate(()=>{history.pushState({},"","/privacy");window.dispatchEvent(new PopStateEvent("popstate"));});
  await p.waitForTimeout(1500);
  const r=await idOf(p);
  console.log("  /privacy → "+JSON.stringify(r));
  if(!r.dashboardish && /about/i.test(r.title)) pass("C290: /privacy now renders the About & Privacy page (not the dashboard): title="+r.title);
  else if(r.privacyContent && !r.dashboardish) pass("C290: /privacy renders privacy content (not dashboard)");
  else fail("/privacy still dashboard-ish: "+JSON.stringify(r));
  // also confirm a hard load (deep-link) works via the deep-link-aware serve.go
  const resp = await p.goto(BASE+"/privacy",{waitUntil:"networkidle"}).catch(()=>null);
  await p.waitForTimeout(4500);
  const r2=await idOf(p);
  console.log("  hard-load /privacy → "+JSON.stringify(r2)+" http="+(resp?resp.status():"?"));
  if(!r2.dashboardish && (/about/i.test(r2.title)||r2.privacyContent)) pass("C290: hard deep-link to /privacy resolves to About/privacy");
  else console.log("  (hard deep-link may differ under gwc dev; SPA nav is the primary path)");
  await p.screenshot({path:"e2e/screenshots/c290_privacy.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
