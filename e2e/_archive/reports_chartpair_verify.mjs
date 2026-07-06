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
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1500);
  // switch to Categories tab
  await p.evaluate(()=>{ const b=[...document.querySelectorAll('[role="radio"],button')].find(x=>/^Categories$/i.test((x.textContent||"").trim())); if(b)b.click(); });
  await p.waitForTimeout(900);
  const wide=await p.evaluate(()=>{
    const g=document.querySelector(".reports-chart-pair");
    if(!g) return null;
    const cs=getComputedStyle(g);
    const cols=cs.gridTemplateColumns.split(" ").filter(Boolean).length;
    const svgs=g.querySelectorAll("svg").length;
    const kids=[...g.children].filter(c=>c.offsetHeight>0).length;
    return { cols, svgs, kids };
  });
  if(!wide){ fail("no .reports-chart-pair on Categories tab"); }
  else{
    console.log("  wide(1440): "+JSON.stringify(wide));
    if(wide.cols===2) pass("chart pair is 2-column at 1440px"); else fail("cols="+wide.cols+" (want 2)");
    if(wide.svgs>=2) pass(wide.svgs+" charts (bar+donut) in the pair"); else fail("only "+wide.svgs+" charts");
  }
  await p.setViewportSize({width:820,height:1000}); await p.waitForTimeout(500);
  const narrow=await p.evaluate(()=>{const g=document.querySelector(".reports-chart-pair"); return g?getComputedStyle(g).gridTemplateColumns.split(" ").filter(Boolean).length:-1;});
  console.log("  narrow(820) cols: "+narrow);
  if(narrow===1) pass("stacks to 1 column at 820px"); else fail("cols="+narrow+" (want 1)");
  await p.setViewportSize({width:1440,height:1000}); await p.waitForTimeout(400);
  await p.screenshot({path:"e2e/screenshots/reports_chartpair.png",fullPage:true});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
