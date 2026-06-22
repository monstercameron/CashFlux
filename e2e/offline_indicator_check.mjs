// L19 gate — "offline indicator / saved-locally reassurance." Asserts the top-bar
// offline pill is hidden while online, appears when the browser goes offline, and
// disappears when back online. Driven by Playwright's context.setOffline, which
// fires the window online/offline events the app listens for.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const visible = (page) => page.locator('[data-testid="offline-indicator"]').isVisible().catch(() => false);
async function waitVisible(page, want, timeoutMs = 6000) {
  for (let w = 0; w < timeoutMs; w += 300) {
    if ((await visible(page)) === want) return true;
    await page.waitForTimeout(300);
  }
  return (await visible(page)) === want;
}

try {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".topbar", { timeout: 60000 });
  await page.waitForTimeout(400);

  // Online at first → no pill.
  if (await visible(page)) fail("offline pill shown while online");

  // Go offline → pill appears.
  await context.setOffline(true);
  if (!(await waitVisible(page, true))) fail("offline pill did not appear when going offline");

  // Back online → pill disappears.
  await context.setOffline(false);
  if (!(await waitVisible(page, false))) fail("offline pill did not disappear when back online");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: offline indicator hidden online, shown offline, hidden again when reconnected.");
} finally {
  await browser.close();
}
