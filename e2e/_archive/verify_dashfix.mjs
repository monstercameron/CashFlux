import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const errs = [];
try {
  const browser = await chromium.launch({ headless: true });
  const p = await (await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 })).newPage();
  p.on("pageerror", e => errs.push(String(e)));
  p.on("console", m => { if (m.type() === "error") errs.push(m.text()); });
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector(".bento .w", { timeout: 60000 });
  await p.waitForTimeout(3500);
  const transferBtn = await p.$$eval('[data-testid="dash-transfer-btn"]', e => e.length);
  const fab = await p.$$eval('.dash-transfer-fab', e => e.length);
  const strip = await p.$$eval('[data-testid^="smart-strip-"]', e => e.length);
  console.log("transfer-btn count:", transferBtn, "| fab count:", fab, "| smart-strip count:", strip, "| console-errors:", errs.length);
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
