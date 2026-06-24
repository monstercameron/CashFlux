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
  await page.waitForSelector(".seg", { timeout: 15000 });
  // seg has the sliding-pill vars + a ::before indicator
  const meta = await page.evaluate(() => {
    const seg = document.querySelector(".seg");
    if (!seg) return null;
    const cs = getComputedStyle(seg);
    const before = getComputedStyle(seg, "::before");
    return { count: cs.getPropertyValue("--seg-count").trim(), idx: cs.getPropertyValue("--seg-idx").trim(),
             beforeW: before.width, beforeTransition: before.transitionProperty };
  });
  ok(meta && meta.count !== "" && +meta.count >= 2, "seg has --seg-count (" + (meta && meta.count) + ")");
  ok(meta && meta.idx !== "", "seg has --seg-idx (" + (meta && meta.idx) + ")");
  ok(meta && /transform/.test(meta.beforeTransition || ""), "indicator transitions transform (slides)");
  // click a different segment, assert --seg-idx changed (the pill moves)
  const btns = await page.locator(".seg .seg-btn");
  const n = await btns.count();
  if (n >= 2) {
    const idx0 = (await page.evaluate(() => getComputedStyle(document.querySelector(".seg")).getPropertyValue("--seg-idx").trim()));
    await btns.nth(n - 1).click();
    await page.waitForTimeout(300);
    const idx1 = (await page.evaluate(() => getComputedStyle(document.querySelector(".seg")).getPropertyValue("--seg-idx").trim()));
    ok(idx1 !== idx0, "clicking a segment moves the pill (--seg-idx " + idx0 + "→" + idx1 + ")");
  }
} catch (e) { ok(false, "exception: " + String(e)); }
finally { await browser.close(); }
console.log(out.join("\n"));
console.log(allPass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(allPass ? 0 : 1);
