import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
  await page.goto(base, { waitUntil: "networkidle" });
  await page.waitForSelector(".topbar", { timeout: 15000 });
  const tb = await page.locator(".topbar").boundingBox();
  const railW = await page.locator("aside.rail").boundingBox();
  console.log("390px topbar height:", Math.round(tb.height), "px");
  console.log("390px rail width:", Math.round(railW.width), "px (collapsed if <=80)");
  // viewport is 844 tall; topbar eating "whole first screen" was ~200px+. Reasonable = under ~120.
  console.log(tb.height <= 120 ? "PASS topbar is compact (<=120px), not eating the screen" : "FAIL topbar " + Math.round(tb.height) + "px too tall");
  console.log(railW.width <= 80 ? "PASS rail auto-collapsed on mobile" : "INFO rail " + Math.round(railW.width) + "px");
} catch (e) { console.log("FAIL " + String(e)); }
finally { await browser.close(); }
