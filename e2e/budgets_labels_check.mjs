// C50 gate — "budget add/edit forms have persistent visible labels". Asserts the
// add form wraps its controls in labeled fields (.labeled-field) with visible text,
// and the inline editor is labeled too. Exits non-zero on any failure.
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

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".labeled-field", { timeout: 60000 });

  const texts = await page.locator(".labeled-field span").allInnerTexts();
  for (const want of ["Name", "Category", "Owner", "Period", "Limit"]) {
    if (!texts.some((t) => t.trim() === want)) fail(`add form missing visible label "${want}" (saw: ${texts.join(", ")})`);
  }

  // Inline editor is labeled too.
  const edit = page.getByRole("button", { name: "Edit" }).first();
  if (await edit.count()) {
    await edit.click();
    await page.waitForSelector(".budget .labeled-field", { timeout: 5000 });
    const editTexts = await page.locator(".budget .labeled-field span").allInnerTexts();
    for (const want of ["Name", "Limit", "Period", "Owner"]) {
      if (!editTexts.some((t) => t.trim() === want)) fail(`inline-edit missing visible label "${want}" (saw: ${editTexts.join(", ")})`);
    }
  } else {
    fail("no budget row Edit button found to verify the inline-edit labels");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: budget add + inline-edit forms carry persistent visible labels.");
} finally {
  await browser.close();
}
