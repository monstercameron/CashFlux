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
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500); // dashboard is the landing route
  const r=await p.evaluate(()=>{
    const c=document.querySelector('[data-testid="cashflow-caption"]');
    if(!c) return {found:false};
    return {found:true, text:c.textContent.trim(), color:getComputedStyle(c).color};
  });
  if(!r.found){ fail("cashflow-caption not found on dashboard"); }
  else {
    console.log("  caption: "+JSON.stringify(r.text)+"  color="+r.color);
    const ok=/kept|more than you earned|broke even/i.test(r.text);
    if(ok) pass("cash-flow widget shows a plain-English takeaway: "+r.text); else fail("caption text unexpected: "+r.text);
  }
  await p.screenshot({path:"e2e/screenshots/r52_cashflow_caption.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
