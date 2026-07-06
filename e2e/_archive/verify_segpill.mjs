import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
let pass = true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  await page.addInitScript(() => localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark", motion: "full" })));
  await page.goto(base + "/appearance", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  await page.waitForSelector(".seg .seg-pill", { timeout: 8000 });
  await page.waitForTimeout(400);
  // Find the theme seg = the .seg that contains a button labelled "Dark".
  const measure = (label) => page.evaluate((lab) => {
    const btns = [...document.querySelectorAll(".seg-btn")];
    const b = btns.find(x => x.textContent.trim() === lab);
    if (!b) return null;
    const seg = b.closest(".seg");
    const pill = seg.querySelector(".seg-pill");
    const active = seg.querySelector(".seg-btn.active");
    return { pillLeft: Math.round(pill.getBoundingClientRect().left),
             activeText: active ? active.textContent.trim() : null,
             pillW: Math.round(pill.getBoundingClientRect().width) };
  }, label);
  const start = await measure("Dark");
  log(start && start.activeText === "Dark", `theme seg active = Dark at start (${start && start.activeText})`);
  const darkLeft = start.pillLeft;
  // Click "System" (rightmost) — a definitely-different option.
  await page.click('.seg-btn:has-text("System")');
  await page.waitForTimeout(500);
  const after = await measure("Dark");
  log(after.activeText === "System", `active moved to System after click (${after.activeText})`);
  log(after.pillLeft > darkLeft + 20, `pill slid right: Dark@${darkLeft} → System@${after.pillLeft}`);
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
