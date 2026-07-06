// L8 gate — "can I afford it?" on Planning. Entering a purchase amount projects
// it against net worth + monthly cash flow (the pure internal/afford engine): a
// small amount shows the projected-balance breakdown, a huge amount reports a
// shortfall. Exits non-zero on any failure.
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
  await page.getByLabel(/Purchase amount/).first().waitFor({ timeout: 60000 });

  const amount = page.getByLabel(/Purchase amount/).first();

  // A token amount: the affordability breakdown (projected-balance stat) should
  // render. (The stat label is CSS-uppercased, so match case-insensitively.)
  await amount.fill("1");
  await page.waitForTimeout(150);
  if (!(await bodyHas(page, /projected balance/i))) fail('entering an amount did not show the projected-balance breakdown');

  // An impossible amount: it should report a shortfall.
  await amount.fill("999999999");
  await page.waitForTimeout(150);
  if (!(await bodyHas(page, /short by/i))) fail('a huge amount did not report a shortfall');

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Planning affordability check projects balance + reports shortfall.");
} finally {
  await browser.close();
}
