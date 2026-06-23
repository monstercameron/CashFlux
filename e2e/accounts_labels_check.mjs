// C49 gate — "account add-form fields have persistent visible labels". Asserts the
// add form wraps its controls in labeled fields (.labeled-field) carrying visible
// text for the key fields, so labels don't vanish on input the way placeholders
// do. Exits non-zero on any failure.
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

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Open the add modal so labeled-field elements are visible.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForTimeout(400);
  await page.waitForSelector(".labeled-field", { timeout: 10000 });

  const count = await page.locator(".labeled-field").count();
  if (count < 5) fail(`expected the add form to have several labeled fields, got ${count}`);

  // The common-path fields carry visible label text.
  const texts = await page.locator(".labeled-field span").allInnerTexts();
  for (const want of ["Name", "Account type", "Owner", "Currency", "Opening balance"]) {
    if (!texts.some((t) => t.trim() === want)) fail(`missing visible label "${want}" (saw: ${texts.join(", ")})`);
  }

  // Close the modal before interacting with the account list behind it.
  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);

  // The inline-edit form is labeled too: open the first account's editor and
  // assert it renders labeled fields with the common labels visible.
  const edit = page.getByRole("button", { name: "Edit" }).first();
  if (await edit.count()) {
    await edit.click();
    await page.waitForSelector(".row-edit .labeled-field", { timeout: 5000 });
    const editTexts = await page.locator(".row-edit .labeled-field span").allInnerTexts();
    for (const want of ["Name", "Owner", "Opening balance"]) {
      if (!editTexts.some((t) => t.trim() === want)) fail(`inline-edit missing visible label "${want}" (saw: ${editTexts.join(", ")})`);
    }
  } else {
    fail("no account row Edit button found to verify the inline-edit labels");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: account add form — ${count} labeled fields; inline-edit form labeled too.`);
} finally {
  await browser.close();
}
