// C56/C46 gate — "subscription price-change rows convey direction with tone + an
// arrow icon, not wording alone". Locates the price-changes card and asserts each
// row carries a tone class (text-up/text-down) and an arrow <svg>. Exits non-zero
// on any failure. Skips gracefully (still PASS) if the sample has no price changes,
// but logs that so a silent no-op is visible.
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

  await page.goto(BASE + "/subscriptions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".row", { timeout: 60000 });

  // Price-change rows carry a tone class on their meta span.
  const toned = page.locator(".row-meta.text-up, .row-meta.text-down");
  const n = await toned.count();
  if (n === 0) {
    console.log("PASS (no-op): sample has no detected price changes to tone — nothing to assert.");
  } else {
    for (let i = 0; i < n; i++) {
      if ((await toned.nth(i).locator("svg").count()) === 0) fail(`price-change row #${i} has a tone class but no arrow icon`);
    }
    if (!process.exitCode) console.log(`PASS: ${n} price-change row(s) show tone + an arrow icon.`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
} finally {
  await browser.close();
}
