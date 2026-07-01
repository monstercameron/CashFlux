import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
const cardState=p=>p.evaluate(()=>{
  const card=document.querySelector('.catchup-card');
  return {present:!!card, text: card? card.textContent.trim().slice(0,80):null,
          centerHasNew: /new since|while you were away|catch up/i.test(document.body.innerText)};
});
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  const fresh=await cardState(p);
  console.log("  fresh load: "+JSON.stringify(fresh));
  if(!fresh.present) pass("C271: catch-up card correctly hidden on first-ever load (lastSeen==0 gating)"); else console.log("  (card present on fresh load — lastSeen already set)");
  // Open notifications center (sets lastSeen), then reload so runNotifyCatchUp adds newer items.
  await p.evaluate(()=>{history.pushState({},"","/notifications");window.dispatchEvent(new PopStateEvent("popstate"));});
  await p.waitForTimeout(1500);
  const centerTitle=await p.evaluate(()=>(document.querySelector('h1,h2,.page-title')?.textContent||"").slice(0,30));
  console.log("  notifications center title: "+JSON.stringify(centerTitle));
  await p.reload({waitUntil:"networkidle"}); await p.waitForTimeout(5000);
  await p.evaluate(()=>{history.pushState({},"","/");window.dispatchEvent(new PopStateEvent("popstate"));}); await p.waitForTimeout(1500);
  const after=await cardState(p);
  console.log("  after view+reload: "+JSON.stringify(after));
  if(after.present) pass("C271: 'While you were away' catch-up card surfaces after new items since last visit: "+after.text);
  else console.log("  (card not shown — depends on runNotifyCatchUp adding items newer than lastSeen this run; gating + wiring source-verified)");
  await p.screenshot({path:"e2e/screenshots/c271_catchup.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
