// L13 gate — "cash runway" on Planning. Adding a large recurring outflow drives the
// projected liquid balance below the buffer, and the runway card warns about the
// dip (via the pure internal/runway + cashflow engines). Exits non-zero on failure.
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

const bodyHas = (page, re) =>
  page.evaluate(({ src, flags }) => new RegExp(src, flags).test(document.body.innerText || ""), {
    src: re.source,
    flags: re.flags,
  });

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[placeholder^="Label (e.g."]', { timeout: 60000 });

  // The runway card exists.
  if (!(await bodyHas(page, /cash runway/i))) fail("the Cash runway card did not render");

  // Add a recurring outflow large enough to breach any starting balance.
  const recForm = page.locator("form").filter({ has: page.locator('input[placeholder^="Label (e.g."]') });
  await recForm.locator('input[placeholder^="Label (e.g."]').fill("Runway breach test");
  await recForm.locator('input[placeholder^="Amount ("]').fill("-99999999");
  await recForm.locator('button[type=submit]').click();

  // The runway should now warn that the balance dips below the buffer.
  await page.waitForFunction(() => /dips below your buffer/i.test(document.body.innerText || ""), { timeout: 10000 }).catch(
    () => fail("a large recurring outflow did not trigger a runway breach warning"),
  );

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Planning cash-runway card warns when recurring outflows breach the buffer.");
} finally {
  await browser.close();
}
