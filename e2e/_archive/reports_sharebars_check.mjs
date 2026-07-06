// C55 gate — "ranked report lists show proportion bars". The spending-by-category,
// top-payees and biggest-expenses lists were plain name+amount rows; they now carry
// a thin share-of-largest bar so the distribution is scannable. Exits non-zero on
// any failure.
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

  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".share-bar", { timeout: 60000 });

  const bars = await page.locator(".share-bar").count();
  if (bars < 3) fail(`expected several proportion bars across the ranked lists, got ${bars}`);

  // The largest row in a list should fill to 100% and others should be narrower —
  // proving the bars are proportional, not decorative.
  const widths = await page.locator(".share-bar > div").evaluateAll((els) =>
    els.map((e) => e.style.width).filter(Boolean)
  );
  if (!widths.some((w) => w === "100%")) fail(`no bar reaches 100% (the list max); widths: ${JSON.stringify(widths.slice(0, 8))}`);
  if (!widths.some((w) => w !== "100%")) fail("every bar is 100% — bars are not proportional");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: reports ranked lists show ${bars} proportion bars (largest fills, others scaled).`);
} finally {
  await browser.close();
}
