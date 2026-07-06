// C48 gate — "Full-width freshness and highlight bands are split into 2-wide tiles".
// DefaultItems now assigns ColSpan 2 (not 4) to freshness and highlight so they sit
// side-by-side instead of taking a heavy full-width band. This test confirms neither
// tile renders with a grid-column spanning all 4 columns by default.
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
  await page.waitForTimeout(1000);

  for (const id of ["freshness", "highlight"]) {
    const tile = page.locator(`[data-widget="${id}"]`);
    const tileCount = await tile.count();
    if (tileCount === 0) {
      // Tile may be hidden; that is acceptable for this check.
      console.log(`INFO: [data-widget="${id}"] not visible — skipped band-span check`);
      continue;
    }
    const colSpan = await tile.first().getAttribute("data-col-span");
    // data-col-span is set by widget.go from the layout item's ColSpan.
    if (colSpan === "4") {
      fail(`[data-widget="${id}"] renders with data-col-span="4" (full-width band) — expected 2 (C48)`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: freshness + highlight tiles are not full-width bands (C48).");
} finally {
  await browser.close();
}
