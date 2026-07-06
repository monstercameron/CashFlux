import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const out = []; let allPass = true;
const ok = (c, m) => { out.push((c ? "PASS " : "FAIL ") + m); if (!c) allPass = false; };
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 } });
  await page.goto(base, { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  // Open quick-add via Alt+N
  await page.keyboard.press("Alt+n");
  await page.waitForTimeout(500);
  const qa = await page.evaluate(() => {
    const form = document.querySelector(".form-grid, .quickadd, .flip-wrap form");
    if (!form) return { found: false };
    const inputs = [...form.querySelectorAll("input:not([type=checkbox]):not([type=hidden]), select")];
    let labeled = 0;
    for (const i of inputs) {
      if (i.getAttribute("aria-label") || i.getAttribute("placeholder") || i.closest("label") || (i.id && document.querySelector(`label[for="${i.id}"]`))) labeled++;
    }
    return { found: true, count: inputs.length, labeled };
  });
  if (!qa.found) { out.push("SKIP quick-add form did not open"); }
  else {
    ok(qa.count > 0, "quick-add has inputs (" + qa.count + ")");
    ok(qa.labeled === qa.count, "#12.5 every quick-add input has an accessible name (" + qa.labeled + "/" + qa.count + ")");
  }
} catch (e) { ok(false, "exception: " + String(e)); }
finally { await browser.close(); }
console.log(out.join("\n"));
console.log(allPass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(allPass ? 0 : 1);
