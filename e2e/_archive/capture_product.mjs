// Captures fresh deliberate product screenshots into docs/screenshots for the
// README / getting-started docs. Loads the sample dataset, then shots each key
// screen at a clean 1280×860 light-theme viewport.
import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const outDir = path.join(process.cwd(), "docs", "screenshots");

const shots = [
  ["/dashboard", "dashboard.png"],
  ["/transactions", "transactions.png"],
  ["/reports", "reports.png"],
  ["/planning", "planning.png"],
  ["/accounts", "accounts.png"],
  ["/budgets", "budgets.png"],
  ["/goals", "goals.png"],
  ["/allocate", "allocate.png"],
];

const browser = await chromium.launch();
const done = [];
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 } });
  await page.addInitScript(() => localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" })));
  // Boot once and load the sample dataset so the screens have real content.
  await page.goto(base + "/accounts", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  const sample = await page.$("text=/load sample/i");
  if (sample) { await sample.click().catch(() => {}); await page.waitForTimeout(800); }
  for (const [route, file] of shots) {
    await page.goto(base + route, { waitUntil: "networkidle" });
    await page.waitForSelector("aside.rail", { timeout: 15000 });
    await page.waitForTimeout(700); // let charts/count-ups settle
    await page.screenshot({ path: path.join(outDir, file) });
    done.push(file);
  }
} catch (e) { console.error("EXCEPTION: " + String(e)); process.exitCode = 1; }
finally { await browser.close(); }
console.log("captured: " + done.join(", "));
console.log(done.length === shots.length ? "RESULT: ALL CAPTURED" : "RESULT: PARTIAL");
