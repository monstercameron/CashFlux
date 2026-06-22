// L14 gate — "Ctrl/Cmd+K toggles the command palette, and running a command
// performs its DIRECT ACTION (not just navigation)." Covers:
//   1. Ctrl+K opens the palette (input visible).
//   2. Ctrl+K again CLOSES it (toggle).
//   3. Ctrl+K opens; Escape closes.
//   4. Direct action — running "Add a transaction" opens the quick-add panel.
//   5. Direct action — running "Collapse / expand sidebar" toggles the rail's
//      collapsed state (a real side effect, not a route change).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const inputVisible = (page) => page.locator("#cf-cmd-input").isVisible().catch(() => false);
const railCollapsed = (page) => page.locator(".rail").first().evaluate((el) => el.classList.contains("collapsed")).catch(() => null);
const rowsText = (page) => page.evaluate(() => Array.from(document.querySelectorAll("[data-cmd-row]")).map((e) => e.textContent || ""));

// Type a query, then click the first row whose text matches `re` (more robust
// than relying on Enter selecting the intended row).
async function runCommand(page, query, re) {
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  await page.fill("#cf-cmd-input", query);
  await page.waitForTimeout(200);
  const rows = page.locator("[data-cmd-row]");
  const n = await rows.count();
  for (let i = 0; i < n; i++) {
    const t = (await rows.nth(i).textContent()) || "";
    if (re.test(t)) { await rows.nth(i).click(); return true; }
  }
  return false;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(600);

  // 1. Ctrl+K opens.
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  if (!(await inputVisible(page))) fail("Ctrl+K did not open the palette");

  // 2. Ctrl+K again closes (toggle).
  await page.keyboard.press("Control+k");
  await page.waitForTimeout(300);
  if (await inputVisible(page)) fail("a second Ctrl+K did not close the palette (toggle)");

  // 3. Ctrl+K opens; Escape closes.
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);
  if (await inputVisible(page)) fail("Escape did not close the palette");

  // 4. Direct action: run the "New transaction" command → quick-add panel opens
  //    (its title is "Add a transaction").
  if (!(await runCommand(page, "add", /New transaction/i))) fail('no "New transaction" command found');
  await page.waitForTimeout(400);
  const quickAdd = await page.getByText("Add a transaction", { exact: false }).count();
  if (quickAdd === 0) fail('running "New transaction" did not open the quick-add panel');
  await page.keyboard.press("Escape"); // close quick-add if it captured focus
  await page.waitForTimeout(300);

  // 5. Direct action: run "Collapse / expand sidebar" → rail collapsed toggles.
  const before = await railCollapsed(page);
  if (before === null) fail("could not read the rail collapsed state");
  if (!(await runCommand(page, "sidebar", /Collapse \/ expand sidebar/i))) fail('no "Collapse / expand sidebar" command found');
  await page.waitForTimeout(400);
  const after = await railCollapsed(page);
  if (after === before) fail(`running the sidebar toggle did not change the rail state (stayed ${before})`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: Ctrl+K toggles the palette (open/close/Escape) and direct actions fire — quick-add opened, sidebar collapsed ${before}→${after}.`);
} finally {
  await browser.close();
}
