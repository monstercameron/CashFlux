import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
try {
  const browser = await chromium.launch({ headless: true });
  const p = await (await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 })).newPage();
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector(".bento .w", { timeout: 60000 });
  await p.waitForTimeout(3000);
  const info = await p.evaluate(() => {
    const btn = document.querySelector('[data-testid="dash-customize"]');
    return {
      count: document.querySelectorAll('[data-testid="dash-customize"]').length,
      inTopbar: btn ? !!btn.closest('.topbar') : false,
      iconOnly: btn ? (btn.textContent.trim() === "") : null,
    };
  });
  console.log("customize:", JSON.stringify(info));
  // Screenshot the topbar region.
  const tb = await p.$('.topbar');
  if (tb) { await tb.screenshot({ path: "e2e/screenshots/topbar.png" }); console.log("shot: e2e/screenshots/topbar.png"); }
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
