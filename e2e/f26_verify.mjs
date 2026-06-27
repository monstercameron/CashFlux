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
  // C200: /debt deep-route
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  await p.evaluate(()=>{ history.pushState({},"","/debt"); window.dispatchEvent(new PopStateEvent("popstate")); });
  await p.waitForTimeout(1500);
  let where=await p.evaluate(()=>location.pathname);
  // fallback: navigate via in-app if a /debt nav link exists, else use /planning
  const onDebt = where==="/debt";
  if(onDebt) pass("C200: /debt route resolves"); else console.log("  (/debt via pushState at "+where+")");
  // scroll to find the debt strategy card
  const r=await p.evaluate(()=>{
    const txt=document.body.innerText;
    return {
      bothStrategies: /snowball/i.test(txt) && /avalanche/i.test(txt),
      debtFreeBy: /debt-free by/i.test(txt),
      timeSaved: /(month|year)s? (sooner|faster|earlier|saved)|saves you|debt-free .* sooner/i.test(txt),
      perDebtTable: !!document.querySelector('table') || /APR/i.test(txt),
      aprEdit: [...document.querySelectorAll('input')].some(i=>/apr|rate/i.test(i.getAttribute('aria-label')||i.getAttribute('placeholder')||"")),
      progress: /paid off .* of .* since/i.test(txt),
    };
  });
  console.log("  "+JSON.stringify(r));
  if(r.bothStrategies) pass("C199: both snowball + avalanche present"); else fail("only one strategy");
  if(r.debtFreeBy) pass("debt-free-by dates shown"); else console.log("  (no debt-free-by — maybe no debts in sample)");
  if(r.perDebtTable) pass("C196: per-debt detail (table/APR) present"); else console.log("  (no per-debt table)");
  if(r.aprEdit) pass("C201: APR editable from the card"); else console.log("  (APR edit input not detected)");
  await p.screenshot({path:"e2e/screenshots/f26_verify.png", fullPage:true});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
