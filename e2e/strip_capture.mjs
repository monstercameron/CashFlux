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
  await p.waitForTimeout(3500);
  // The strip card's testid starts with "smart-strip-" but excludes the buttons.
  const el = await p.$('[data-testid^="smart-strip-"]:not(button)');
  if (!el) { console.log("no strip card found"); }
  else {
    await el.scrollIntoViewIfNeeded();
    await p.waitForTimeout(300);
    await el.screenshot({ path: "e2e/screenshots/smart_strip.png" });
    console.log("shot: e2e/screenshots/smart_strip.png");
  }
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
