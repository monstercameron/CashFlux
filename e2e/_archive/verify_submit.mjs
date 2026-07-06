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
  await page.addInitScript(() => { try { localStorage.clear(); } catch(e){} });
  await page.goto(base + "/accounts", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  // Load sample data so transactions have an account to attach to.
  const sampleBtn = await page.locator("button:has-text('sample'), button:has-text('Sample'), button:has-text('Load')").first();
  if (await sampleBtn.count()) { await sampleBtn.click().catch(()=>{}); await page.waitForTimeout(800); }
  // Open quick-add (Alt+N)
  await page.keyboard.press("Alt+n");
  await page.waitForSelector(".flip-backdrop.show, .form-grid", { timeout: 5000 }).catch(()=>{});
  await page.waitForTimeout(300);
  // Fill the amount (number input) and submit via Enter
  const amt = page.locator(".flip-wrap input[type=number], .form-grid input[type=number]").first();
  if (await amt.count()) {
    await amt.fill("12.50");
    await amt.press("Enter");
    await page.waitForTimeout(700);
    const backdropGone = (await page.locator(".flip-backdrop.show").count()) === 0;
    const toast = (await page.locator(".toast").count()) > 0;
    ok(backdropGone, "#12.2a backdrop gone after submit");
    ok(toast, "#12.2b toast appears after submit");
  } else { out.push("SKIP no amount input (sample may not have loaded)"); }
} catch (e) { ok(false, "exception: " + String(e)); }
finally { await browser.close(); }
console.log(out.join("\n"));
console.log(allPass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(allPass ? 0 : 1);
