// Captures README screenshots: boot the running app, then click through the rail
// (deep links 404 under gwc dev, so navigate in-app) and shoot each screen into
// docs/screenshots/. Not a pass/fail test.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const OUT = path.join(__dirname, "..", "docs", "screenshots");

const shots = [
  { title: null, file: "dashboard.png" },          // home
  { title: "Transactions", file: "transactions.png" },
  { title: "Reports", file: "reports.png" },
  { title: "Planning", file: "planning.png" },
  { title: "Allocate", file: "allocate.png" },
];

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);

  for (const s of shots) {
    if (s.title) {
      await page.locator(`nav a[title="${s.title}"]`).first().click();
      await page.waitForTimeout(900);
    }
    await page.screenshot({ path: path.join(OUT, s.file) });
    console.log("wrote", s.file);
  }
  console.log("page errors:", errors.length ? errors.join(" | ") : "none");
} finally {
  await browser.close();
}
