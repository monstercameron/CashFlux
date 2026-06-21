// Dashboard "Needs attention" digest: it renders as the top widget, its gear
// settings control which sources show (turning all sources off empties it), and
// the dashboard layout manager now lives in the Settings modal. Exits non-zero on
// any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(900);

  // 1) It's the top widget, anchored at grid row 1.
  const first = page.locator(".bento .w").first();
  const title = (await first.locator(".wtitle, h3, .title").first().textContent().catch(() => "") || "").trim();
  if (!title.includes("Needs attention")) fail(`top widget is "${title}", want "Needs attention"`);
  const gridRow = await first.evaluate((e) => getComputedStyle(e).gridRowStart);
  if (gridRow !== "1") fail(`attention widget grid-row-start = ${gridRow}, want 1`);

  // Sample data has urgent items, so the digest should show some.
  const before = await page.locator(".attention-item").count();
  if (before < 1) fail("expected at least one attention item from sample data");

  // 2) Gear settings drive which sources show — turn every source off → empty.
  await first.locator(".gear-inline").click();
  await page.waitForTimeout(500);
  for (const label of ["Bills due soon", "Budget alerts (near or over)", "Stale account balances", "Overdue & high-priority to-dos", "Biggest spending spike"]) {
    const row = page.locator(".toggle-row", { hasText: label }).first();
    if ((await row.count()) === 0) { fail(`missing settings toggle: ${label}`); continue; }
    await row.locator("button, input[type=checkbox], [role=switch]").first().click();
    await page.waitForTimeout(120);
  }
  await page.waitForTimeout(400);
  if ((await page.locator(".attention-item").count()) !== 0) fail("turning all sources off should empty the digest");
  if (!(await page.evaluate(() => document.body.innerText.includes("All clear")))) fail("empty digest should show the 'All clear' message");

  // 3) The dashboard layout manager now lives in Settings (mode select + Reset),
  // not in a wasted dashboard header cell.
  const page2 = await (await browser.newContext()).newPage();
  await page2.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page2.waitForSelector(".bento", { timeout: 60000 });
  await page2.waitForTimeout(700);
  await page2.locator("button.hh").first().click(); // the household card opens global Settings
  await page2.waitForTimeout(700);
  const hasLayout = await page2.evaluate(() => {
    const sectionLabel = [...document.querySelectorAll(".set-label")].some((e) => e.textContent.trim() === "Dashboard layout");
    const t = document.body.innerText;
    return sectionLabel && t.includes("Reset layout") && t.includes("Custom layout");
  });
  if (!hasLayout) fail("Settings should contain the Dashboard layout controls (mode select + Reset)");

  if (!process.exitCode) console.log("PASS: attention widget is the top tile, its gear toggles which sources show, and the layout manager moved to Settings.");
} finally {
  await browser.close();
}
