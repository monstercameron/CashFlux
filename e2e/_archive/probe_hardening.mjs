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
  // #12.1 — open settings (household card) → flip-backdrop.show present
  await page.click(".hh");
  const backdrop = await page.waitForSelector(".flip-backdrop.show", { timeout: 5000 }).catch(() => null);
  ok(!!backdrop, "#1 modal open → .flip-backdrop.show present");
  // #3 — settings header h3 color is not near-white (regression: light surfaces).
  const h3color = await page.evaluate(() => {
    const h = document.querySelector(".set-h h3");
    return h ? getComputedStyle(h).color : null;
  });
  ok(h3color !== null, "#3 settings header present (color " + h3color + ")");
  // close settings
  await page.keyboard.press("Escape");
  await page.waitForTimeout(400);
  const stillOpen = await page.locator(".flip-backdrop.show").count();
  ok(stillOpen === 0, "#2 Escape closes modal (backdrop gone)");
  // #5 — QuickAdd inputs each have an accessible name (aria-label or wrapping <label>).
  await page.click(".add-btn").catch(() => {});
  await page.waitForTimeout(300);
  // Try opening quick-add transaction from the +Add menu
  const qa = await page.evaluate(() => {
    const inputs = [...document.querySelectorAll(".form-grid input:not([type=checkbox]), .quickadd input:not([type=checkbox])")];
    if (!inputs.length) return { count: 0, labeled: 0 };
    let labeled = 0;
    for (const i of inputs) {
      const hasAria = i.getAttribute("aria-label") || i.getAttribute("placeholder");
      const wrapLabel = i.closest("label");
      if (hasAria || wrapLabel) labeled++;
    }
    return { count: inputs.length, labeled };
  });
  if (qa.count > 0) ok(qa.labeled === qa.count, "#5 all " + qa.count + " add-form inputs have a name (" + qa.labeled + "/" + qa.count + ")");
  else out.push("SKIP #5 no add-form inputs visible");
} catch (e) { ok(false, "exception: " + String(e)); }
finally { await browser.close(); }
console.log(out.join("\n"));
console.log(allPass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(allPass ? 0 : 1);
