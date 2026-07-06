// C66 gate — "a failed artifact import is no longer silent". Importing a bad
// (empty) CSV used to do nothing; it now surfaces the error in the toast. Exits
// non-zero on any failure.
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

  await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /import csv/i }).waitFor({ timeout: 60000 });

  // pickFile() opens a native chooser via input.click(); feed it an empty CSV so
  // ParseCSV fails and the error must surface.
  page.once("filechooser", async (fc) => {
    await fc.setFiles({ name: "empty.csv", mimeType: "text/csv", buffer: Buffer.from("") });
  });
  await page.getByRole("button", { name: /import csv/i }).click();

  // The error should appear in the app toast (was previously swallowed).
  await page.waitForSelector(".toast-msg", { timeout: 8000 }).catch(() => fail("no toast appeared for the failed CSV import"));
  const msg = (await page.locator(".toast-msg").first().innerText().catch(() => "")) || "";
  if (!/csv/i.test(msg)) fail(`toast should explain the CSV failure, got "${msg}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: failed CSV import surfaces a toast ("${msg.trim()}").`);
} finally {
  await browser.close();
}
