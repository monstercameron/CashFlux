// C52 gate — "the priority + due-date controls are labelled". Both were previously
// unlabelled (no aria-label, no visible label). Asserts the add form now shows
// visible "Priority" and "Due date" labels via .labeled-field. Exits non-zero on
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

  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#task-add", { timeout: 60000 });
  await page.waitForSelector(".labeled-field", { timeout: 5000 });

  const texts = (await page.locator(".labeled-field span").allInnerTexts()).map((t) => t.trim());
  for (const want of ["Priority", "Due date"]) {
    if (!texts.includes(want)) fail(`add form should show a visible "${want}" label (saw: ${texts.join(", ")})`);
  }
  // The controls also carry matching aria-labels.
  if ((await page.locator('select[aria-label="Priority"]').count()) === 0) fail("priority select missing aria-label");
  if ((await page.locator('input[aria-label="Due date"]').count()) === 0) fail("due-date input missing aria-label");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: to-do priority + due-date controls are labelled (visible + aria).");
} finally {
  await browser.close();
}
