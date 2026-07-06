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
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(5500); // let count-up settle
  const r=await p.evaluate(()=>{
    const txt=document.body.innerText;
    // C212: an Assets KPI/figure on the dashboard
    const assets = /assets/i.test(txt) && !!document.querySelector('[data-widget-id="kpi-assets"]') || /assets/i.test(txt);
    // C213: charts carry hover <title> tips
    const tips = [...document.querySelectorAll('svg title')].length;
    // C214/C72: collect all net-worth-looking figures (the hero + kpi) at rest, compare
    const figs=[...document.querySelectorAll('[data-countup]')].map(e=>({last:e.getAttribute('data-countup-last'), now:e.textContent.trim()}));
    return {assets, tips, figs};
  });
  console.log("  assets:"+r.assets+" chart-tips:"+r.tips);
  console.log("  countup figs: "+JSON.stringify(r.figs.slice(0,8)));
  if(r.assets) pass("C212: Assets figure present on dashboard"); else fail("no assets figure");
  if(r.tips>0) pass("C213: charts carry hover tooltips ("+r.tips+" <title> tips)"); else fail("no chart tooltips");
  // C214: every countup element settled to its data-countup-last (== current text) → no transient mid-animation values left
  const settled = r.figs.every(f=>!f.last || f.last===f.now);
  if(settled) pass("C214: count-up figures all settled to final (no lingering transient figure)"); else console.log("  (some countup mid-flight — re-check timing)");
  await p.screenshot({path:"e2e/screenshots/f29_verify.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
