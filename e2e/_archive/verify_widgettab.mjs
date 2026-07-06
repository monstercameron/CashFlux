import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/dashboard",{waitUntil:"networkidle"}); await p.waitForSelector(".bento",{timeout:15000}); await p.waitForTimeout(700);
  // 1) Exactly ONE draggable tile is a Tab stop (tabindex=0); the rest are -1.
  const counts = await p.evaluate(()=>{ const tiles=[...document.querySelectorAll('[data-widget][draggable="true"]')];
    return { total: tiles.length, tab0: tiles.filter(t=>t.getAttribute("tabindex")==="0").length, tabneg: tiles.filter(t=>t.getAttribute("tabindex")==="-1").length }; });
  log(counts.total>=8, `dashboard has ${counts.total} draggable tiles`);
  log(counts.tab0===1, `exactly ONE tile is a Tab stop (got ${counts.tab0}), not ${counts.total}`);
  log(counts.tabneg===counts.total-1, `the other ${counts.tabneg} tiles are removed from the tab order`);
  // 2) Arrow moves focus between tiles (roving).
  const firstID = await p.evaluate(()=>{ const t=document.querySelector('[data-widget][draggable="true"][tabindex="0"]'); t.focus(); return t.getAttribute("data-widget"); });
  await p.keyboard.press("ArrowRight"); await p.waitForTimeout(200);
  const afterArrow = await p.evaluate(()=>document.activeElement.getAttribute && document.activeElement.getAttribute("data-widget"));
  log(afterArrow && afterArrow!==firstID, `ArrowRight moved focus to another tile (${firstID} -> ${afterArrow})`);
  // 3) Space grabs the focused tile (aria-grabbed=true).
  await p.keyboard.press("Space"); await p.waitForTimeout(200);
  const grabbed = await p.evaluate(()=>document.activeElement.getAttribute && document.activeElement.getAttribute("aria-grabbed"));
  log(grabbed==="true", `Space grabbed the tile for keyboard move (aria-grabbed=${grabbed})`);
  // 4) Escape releases.
  await p.keyboard.press("Escape"); await p.waitForTimeout(200);
  const released = await p.evaluate(()=>document.activeElement.getAttribute && document.activeElement.getAttribute("aria-grabbed"));
  log(released==="false", `Escape released the grab (aria-grabbed=${released})`);
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
