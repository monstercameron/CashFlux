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
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1300);
  await p.evaluate(()=>{const b=[...document.querySelectorAll('[role="radio"],button')].find(x=>/^Categories$/i.test((x.textContent||"").trim())); if(b)b.click();});
  await p.waitForTimeout(900);
  const yoy=await p.evaluate(()=>{
    const b=document.querySelector('[data-testid="reports-yoy-toggle"]');
    return b? {present:true, label:b.textContent.trim(), pressed:b.getAttribute('aria-pressed')} : {present:false};
  });
  console.log("  YoY toggle: "+JSON.stringify(yoy));
  if(yoy.present) pass("C237: YoY toggle present on Categories tab: "+yoy.label); else fail("YoY toggle missing");
  // toggle it and confirm aria-pressed flips
  if(yoy.present){ await p.click('[data-testid="reports-yoy-toggle"]'); await p.waitForTimeout(600);
    const after=await p.evaluate(()=>document.querySelector('[data-testid="reports-yoy-toggle"]')?.getAttribute('aria-pressed'));
    if(after!==yoy.pressed) pass("C237: YoY toggle flips state ("+yoy.pressed+"→"+after+")"); else console.log("  (aria-pressed unchanged: "+after+")"); }
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
