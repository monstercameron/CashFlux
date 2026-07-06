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
  const present=await p.evaluate(()=>{const a=document.querySelector('[data-testid="income-drill"]'); return a?{href:a.getAttribute('href'),text:a.textContent.trim()}:null;});
  console.log("  income drill: "+JSON.stringify(present));
  if(present && /transactions/.test(present.href)) pass("R52(b): income card has 'View transactions' drill"); else fail("income drill missing");
  if(present){ await p.click('[data-testid="income-drill"]'); await p.waitForTimeout(1100);
    const where=await p.evaluate(()=>location.pathname);
    if(where==="/transactions") pass("income drill navigates to /transactions"); else fail("nav target: "+where); }
  await p.screenshot({path:"e2e/screenshots/r52_income_drill.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
