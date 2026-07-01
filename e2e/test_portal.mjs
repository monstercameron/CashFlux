import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const SD = "C:/Users/mreca/AppData/Local/Temp/claude/C--Users-mreca-Desktop/5aacab8d-c372-4a7d-97dc-bfed206563c6/scratchpad";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1000 } });
const results=[]; const check=(n,c,d="")=>{results.push(!!c);console.log((c?"PASS ":"FAIL ")+n+(d?" — "+d:""));};
const errs=[]; p.on("pageerror",e=>errs.push(String(e)));
await p.goto("http://127.0.0.1:8091/",{waitUntil:"domcontentloaded"}); await p.waitForSelector("#app .bento",{timeout:30000}).catch(()=>{}); await p.waitForTimeout(1000);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto("http://127.0.0.1:8091/goals",{waitUntil:"domcontentloaded"}); await p.waitForTimeout(1200);
await p.locator('[data-testid^="smart-tip-"]').first().locator('button').first().click(); await p.waitForTimeout(600);
const r = await p.evaluate(()=>{
  const pop=document.querySelector('.smart-tip-pop'); if(!pop) return {found:false};
  const rc=pop.getBoundingClientRect();
  const parentIsBody = pop.parentElement===document.body;
  // hit-test corners + center (topmost element there must be the pop → nothing paints over it)
  const pts=[[rc.left+6,rc.top+6],[rc.right-6,rc.top+6],[rc.left+6,rc.bottom-6],[rc.right-6,rc.bottom-6],[(rc.left+rc.right)/2,(rc.top+rc.bottom)/2]];
  const allHit=pts.every(([x,y])=>{const el=document.elementFromPoint(x,y);return el&&(el===pop||pop.contains(el));});
  return { found:true, parentIsBody, allHit, onScreen: rc.left>=0&&rc.top>=0&&rc.right<=innerWidth&&rc.bottom<=innerHeight, rect:{t:Math.round(rc.top),bt:Math.round(rc.bottom)} };
});
check("P1 popover is portaled to <body> (escapes the tile stacking context)", r.parentIsBody);
check("P2 popover fully on-screen", r.onScreen, JSON.stringify(r.rect));
check("P3 nothing paints over the popover (not covered by the next section)", r.allHit);
check("P4 no page errors", errs.length===0, errs.slice(0,2).join(" | "));
// close by clicking away, confirm the portal node is removed (no leak)
await p.mouse.click(700, 300); await p.waitForTimeout(400);
check("P5 popover node removed on dismiss (no orphan in body)", await p.locator('.smart-tip-pop').count()===0);
await p.locator('[data-testid^="smart-tip-"]').first().locator('button').first().click(); await p.waitForTimeout(500);
await p.screenshot({ path: SD+"/portal.png" });
console.log(`\nRESULT: ${results.filter(Boolean).length}/${results.length}`);
await b.close();
