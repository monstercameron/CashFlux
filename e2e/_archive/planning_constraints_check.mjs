// C53 gate — "planning number fields are constrained". The plans horizon and
// one-time-month inputs must be >= 1, and the payoff/debt-strategy money inputs
// >= 0, so bad values are caught at the field. Exits non-zero on any failure.
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

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="number"]', { timeout: 60000 });
  await page.waitForTimeout(400);

  const mins = await page.locator('input[type="number"]').evaluateAll((els) => els.map((e) => e.getAttribute("min")));
  const min1 = mins.filter((m) => m === "1").length;
  const min0 = mins.filter((m) => m === "0").length;
  // horizon + one-time month → at least two min="1"; payoff balance/payment/extra +
  // debt-strategy extra → at least four min="0".
  if (min1 < 2) fail(`expected >=2 inputs with min="1" (horizon, one-time month), got ${min1}`);
  if (min0 < 4) fail(`expected >=4 inputs with min="0" (payoff + debt-strategy money), got ${min0}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: planning constraints — ${min1} inputs min=1, ${min0} inputs min=0.`);
} finally {
  await browser.close();
}
