import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const out = [];
let allPass = true;
const ok = (c, m) => { out.push((c ? "PASS " : "FAIL ") + m); if (!c) allPass = false; };
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(base, { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  ok(await page.locator("aside.rail").count() > 0, "rail nav rendered");
  ok(await page.locator("#cf-page-view").count() > 0, "page view rendered");
  ok(await page.locator(".topbar").count() > 0, "topbar rendered");
  ok(await page.locator(".breadcrumb").count() > 0, "breadcrumb present (GX1-F8)");
  ok(errors.length === 0, "no console pageerrors (" + errors.length + ")");
  for (const route of ["/accounts","/budgets","/transactions","/goals","/reports"]) {
    await page.goto(base + route, { waitUntil: "networkidle" });
    await page.waitForSelector("aside.rail", { timeout: 10000 });
    ok(errors.length === 0, "route " + route + " loaded, no errors");
  }
} catch (e) { ok(false, "exception: " + String(e)); }
finally { await browser.close(); }
console.log(out.join("\n"));
console.log(allPass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(allPass ? 0 : 1);
