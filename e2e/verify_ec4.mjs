import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/accounts",{waitUntil:"networkidle"}); await p.waitForSelector("aside.rail",{timeout:20000});
  const s=await p.$("text=/load sample/i"); if(s){await s.click().catch(()=>{}); await p.waitForTimeout(800);}
  await p.goto(base+"/dashboard",{waitUntil:"networkidle"}); await p.waitForSelector(".bento",{timeout:15000}); await p.waitForTimeout(800);
  const hero=await p.$(".home-hero");
  log(!!hero, "EC4 home-hero band renders on dashboard");
  const order=await p.evaluate(()=>{const h=document.querySelector(".home-hero"),bt=document.querySelector(".bento");if(!h||!bt)return"missing";return(h.compareDocumentPosition(bt)&Node.DOCUMENT_POSITION_FOLLOWING)?"above":"below";});
  log(order==="above", `sits above the bento (${order})`);
  const hasStats=hero? (await hero.$$(".home-hero-stats, .home-hero-stat, [class*=hero-nw]")).length>0 : false;
  log(hasStats, "shows glanceable summary (net worth / stats)");
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
