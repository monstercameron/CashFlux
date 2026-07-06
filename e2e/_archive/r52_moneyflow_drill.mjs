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
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1500);
  const a=await p.evaluate(()=>{const e=document.querySelector('[data-testid="moneyflow-drill"]'); return e?{href:e.getAttribute('href')}:null;});
  console.log("  moneyflow drill: "+JSON.stringify(a));
  if(a && /transactions/.test(a.href)) pass("R52(b): Money flow card has 'View transactions' drill"); else fail("moneyflow drill missing");
  if(a){ await p.click('[data-testid="moneyflow-drill"]'); await p.waitForTimeout(1100);
    const where=await p.evaluate(()=>location.pathname);
    if(where==="/transactions") pass("moneyflow drill navigates to /transactions"); else fail("nav: "+where); }
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,3).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
