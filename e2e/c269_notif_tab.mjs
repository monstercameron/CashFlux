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
  await p.evaluate(()=>{const b=document.querySelector('button.hh'); if(b)b.click();});
  await p.waitForTimeout(900);
  const r=await p.evaluate(()=>{
    const navBtns=[...document.querySelectorAll('.set-section-nav button')].map(b=>b.textContent.trim());
    const hasNotif = navBtns.some(t=>/notification/i.test(t));
    // matching section heading exists?
    const heading=[...document.querySelectorAll('.set-label')].some(e=>/^notifications$/i.test(e.textContent.trim()));
    return {navBtns, hasNotif, heading};
  });
  console.log("  nav buttons: "+JSON.stringify(r.navBtns));
  if(r.hasNotif) pass("C269: 'Notifications' jump-to tab present in Settings nav"); else fail("no Notifications jump tab");
  if(r.heading) pass("matching 'Notifications' section heading exists (jump target valid)"); else fail("no Notifications .set-label heading — jump would be a no-op");
  await p.screenshot({path:"e2e/screenshots/c269_notif_tab.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
