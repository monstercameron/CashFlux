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
  const info=await p.evaluate(()=>{
    const tile=document.querySelector('[data-widget-id="kpi-safetospend"]')||
               [...document.querySelectorAll(".w")].find(w=>/safe to spend/i.test(w.textContent||""));
    if(!tile) return {found:false};
    const fig=tile.querySelector(".fig");
    const sub=tile.querySelector(".t-caption");
    return {
      found:true,
      title: (tile.querySelector(".wtitle, .whead, h3, .t-title")?.textContent||tile.textContent.slice(0,40)).trim(),
      fig: fig?fig.textContent.trim():null,
      figColor: fig?getComputedStyle(fig).color:null,
      sub: sub?sub.textContent.trim():null,
    };
  });
  if(!info.found){ fail("kpi-safetospend tile not found on dashboard"); }
  else{
    console.log("  "+JSON.stringify(info));
    if(info.fig && /[\d$]/.test(info.fig)) pass("safe-to-spend tile shows a money figure: "+info.fig);
    else fail("no money figure in tile: "+info.fig);
    if(info.sub && info.sub.length>0) pass("tile has a sub-caption: "+JSON.stringify(info.sub));
  }
  await p.screenshot({path:"e2e/screenshots/s2s_kpi.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
