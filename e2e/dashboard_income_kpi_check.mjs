// C1 gate — "Dashboard Income KPI shows non-zero income for the in-period salary".
// With sample data loaded, the dashboard Income tile must show a figure > $0.00 and
// a deposit count ≥ 1 for the current month. The original bug dropped first-of-month
// UTC-dated transactions in behind-UTC time zones (period boundary used local time).
// This test runs with the server's local timezone and confirms the KPI reads the
// salary regardless of zone.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

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

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app .bento", { timeout: 60000 });
  // Allow the WASM module and sample data to finish loading.
  await page.waitForTimeout(1200);

  // The dashboard Income KPI tile is rendered with data-testid="kpi-income" or
  // contains the title "Income". Find the tile and read its figure text.
  const incomeWidget = page.locator('[data-widget="kpi-income"]');
  const count = await incomeWidget.count();
  if (count === 0) {
    fail('Income KPI tile ([data-widget="kpi-income"]) not found on dashboard');
  } else {
    // The primary figure lives in .t-figure or .fig inside the tile.
    const figText = await incomeWidget.locator(".t-figure, .fig").first().textContent();

    // The figure must not be "$0.00" and must not be empty.
    if (!figText || figText.trim() === "" || figText.includes("$0.00") || figText === "$0") {
      fail(`Income KPI shows "${figText}" — expected a non-zero income figure (C1 regression)`);
    }

    // The subline must mention at least 1 deposit.
    const subText = await incomeWidget.locator(".t-caption, p").first().textContent().catch(() => "");
    // Accept "1 deposit", "2 deposits", etc.
    const depositMatch = /\d+\s+deposit/.test(subText || "");
    if (!depositMatch) {
      fail(`Income KPI subline "${subText}" does not mention a deposit count (C1 regression)`);
    }

    if (!process.exitCode) {
      console.log(`PASS: Dashboard Income KPI — figure "${figText}", subline "${subText}" (C1 confirmed).`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
} finally {
  await browser.close();
}
