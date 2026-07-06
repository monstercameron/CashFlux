import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
let pass = true; const log = (c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
const check = async (w) => {
  const page = await browser.newPage({ viewport: { width: w, height: 900 } });
  await page.goto(base + "/transactions", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  const s = await page.$("text=/load sample/i"); if (s) { await s.click().catch(()=>{}); await page.waitForTimeout(600); }
  const r = await page.evaluate((vw) => {
    const t = document.querySelector(".txn-table");
    if (!t) return { mode: "none" };
    const thead = t.querySelector("thead");
    const headVisible = thead ? getComputedStyle(thead).display !== "none" : false;
    const right = Math.max(0, ...[...t.querySelectorAll("tr.row, tbody td")].map(e => e.getBoundingClientRect().right));
    return { mode: headVisible ? "table" : "cards", right: Math.round(right), vw };
  }, w);
  await page.close();
  return r;
};
try {
  const r1000 = await check(1000); log(r1000.mode === "cards", `@1000px → card layout (was clip band), got ${r1000.mode}`);
  const r1150 = await check(1150); log(r1150.mode === "cards", `@1150px → card layout (was clip band), got ${r1150.mode}`);
  const r1300 = await check(1300); log(r1300.mode === "table", `@1300px → real table, got ${r1300.mode}`);
  log(r1300.right <= 1300, `@1300px table right edge ${r1300.right} ≤ 1300 viewport (no clip)`);
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
