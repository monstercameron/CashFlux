import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
try {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const p = await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1800);

  const wide = await p.evaluate(()=>{
    const g=document.querySelector(".reports-grid");
    if(!g) return null;
    const cs=getComputedStyle(g);
    const cols=cs.gridTemplateColumns.split(" ").filter(Boolean).length;
    // children that are actual section cards (skip empty fragments)
    const kids=[...g.children].filter(c=>c.offsetHeight>0).length;
    // confirm Sankey (Money flow) is OUTSIDE the grid (full width sibling)
    const sankeyInGrid = !!g.querySelector(".mermaid, svg") && /Money flow/i.test(g.textContent);
    return { cols, kids, gridWidth: Math.round(g.getBoundingClientRect().width), sankeyInGrid };
  });
  if(!wide) fail("no .reports-grid found on /reports");
  else {
    console.log("  wide(1440): "+JSON.stringify(wide));
    if(wide.cols===2) pass("2-column grid at 1440px"); else fail("cols="+wide.cols+" at 1440 (want 2)");
    if(wide.kids>=2) pass(wide.kids+" section cards in grid"); else fail("only "+wide.kids+" cards");
    if(!wide.sankeyInGrid) pass("Money-flow Sankey stays full-width outside the grid"); else fail("Sankey leaked into grid");
  }

  // Narrow viewport → single column.
  await p.setViewportSize({ width: 900, height: 1000 });
  await p.waitForTimeout(500);
  const narrow = await p.evaluate(()=>{
    const g=document.querySelector(".reports-grid");
    return g? getComputedStyle(g).gridTemplateColumns.split(" ").filter(Boolean).length : -1;
  });
  console.log("  narrow(900) cols: "+narrow);
  if(narrow===1) pass("1-column grid at 900px (responsive)"); else fail("cols="+narrow+" at 900 (want 1)");

  await p.setViewportSize({ width: 1440, height: 1000 });
  await p.waitForTimeout(400);
  await p.screenshot({ path: "e2e/screenshots/reports_grid_2col.png", fullPage:true });
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR: "+e));fail("console errors");}
} catch(e){ fail("exception: "+e.message); } finally { await browser.close(); }
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
