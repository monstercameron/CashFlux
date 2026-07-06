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
  await p.waitForTimeout(4500);
  await p.click('a[href="/budgets"]'); await p.waitForTimeout(1800);
  const info=await p.evaluate(()=>{
    const lines=[...document.querySelectorAll('[data-testid="budget-prorated"]')].map(e=>e.textContent.trim());
    return { count: lines.length, samples: lines.slice(0,3) };
  });
  console.log("  prorated lines: "+JSON.stringify(info));
  if(info.count>0 && /~\D*[\d]/.test(info.samples[0])) pass("per-category prorated line renders: "+info.samples[0]);
  else fail("no prorated line found (count="+info.count+")");
  await p.screenshot({path:"e2e/screenshots/c143_prorated.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
