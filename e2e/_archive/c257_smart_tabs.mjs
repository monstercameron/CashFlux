// C257 E2E — /smart hub tab split: "Insights" (default) + "Manage".
// Verifies the tab bar renders, Insights is the default, and clicking Manage
// switches content to the manage catalog.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/smart", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-testid="smart-hub"]', { timeout: 60000 });
  await page.waitForTimeout(700);

  // Both tab buttons must be present.
  const insightsTab = page.locator('[data-testid="smart-tab-insights"]');
  const manageTab = page.locator('[data-testid="smart-tab-manage"]');

  if (!(await insightsTab.isVisible())) fail("smart-tab-insights button is not visible");
  if (!(await manageTab.isVisible())) fail("smart-tab-manage button is not visible");

  // Insights tab is selected by default.
  const insightsSel = await insightsTab.getAttribute("aria-selected");
  if (insightsSel !== "true") fail(`insights tab should be aria-selected=true by default, got "${insightsSel}"`);
  const manageSel = await manageTab.getAttribute("aria-selected");
  if (manageSel !== "false") fail(`manage tab should be aria-selected=false by default, got "${manageSel}"`);

  // Insights section is visible; manage section is not.
  const insightsSection = page.locator('[data-testid="smart-insights"]');
  const manageSection = page.locator('[data-testid="smart-manage"]');

  if (!(await insightsSection.isVisible())) fail("smart-insights section should be visible on Insights tab");
  if (await manageSection.isVisible()) fail("smart-manage section should NOT be visible on Insights tab");

  // Screenshot: insights tab.
  const ssDir = path.join(__dirname, "screenshots");
  if (!fs.existsSync(ssDir)) fs.mkdirSync(ssDir, { recursive: true });
  await page.screenshot({ path: path.join(ssDir, "c257_smart_tabs.png") });

  // Click Manage tab — content should switch.
  await manageTab.click();
  await page.waitForTimeout(400);

  const manageSelAfter = await manageTab.getAttribute("aria-selected");
  if (manageSelAfter !== "true") fail(`manage tab should be aria-selected=true after click, got "${manageSelAfter}"`);

  if (await insightsSection.isVisible()) fail("smart-insights section should NOT be visible on Manage tab");
  if (!(await manageSection.isVisible())) fail("smart-manage section should be visible on Manage tab");

  // At least one feature row should exist in the manage catalog.
  const featureRows = page.locator('[data-testid^="smart-feature-"]');
  const rowCount = await featureRows.count();
  if (rowCount === 0) fail("expected at least one smart-feature-* row in the Manage catalog");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(`PASS: /smart tab split works — Insights default, Manage shows ${rowCount} feature rows.`);
} finally {
  await browser.close();
}
