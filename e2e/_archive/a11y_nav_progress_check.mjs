// L34/L19/L35 gate — rail nav items are real links (href + keyboard-focusable)
// with aria-current on the active one; budget/goal progress bars expose
// role=progressbar with aria-valuenow.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav a', { timeout: 60000 });
  await page.waitForTimeout(500);

  // Nav anchors have real hrefs.
  const hrefs = await page.$$eval('nav a.nv, nav a.nav', (as) => as.map((a) => a.getAttribute("href")));
  const withHref = hrefs.filter((h) => h && h.length > 0);
  if (withHref.length === 0) fail(`nav links have no href (got ${hrefs.length} nav anchors, 0 with href)`);

  // The active item has aria-current="page".
  const current = await page.$$eval('nav a[aria-current="page"]', (as) => as.length);
  if (current === 0) fail('no nav item has aria-current="page"');

  // Budget progress bars expose role=progressbar with aria-valuenow.
  const bars = await page.$$eval('[role="progressbar"]', (els) => els.filter((e) => e.getAttribute("aria-valuenow") !== null).length);
  if (bars === 0) fail("no role=progressbar with aria-valuenow on the Budgets screen");

  if (!process.exitCode) console.log(`PASS: ${withHref.length} nav links with href, aria-current present, ${bars} accessible progress bars.`);
} finally {
  await browser.close();
}
