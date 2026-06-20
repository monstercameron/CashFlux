// C54 gate — "an allocate suggestion shows its score once". The row used to print
// the score twice (head "60%" and a "Score 60%" sub-line) with a manual " · "
// separator. Now the score is in the head + the progress bar only, and the
// breakdown is the lone sub-line. Exits non-zero on any failure.
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

  await page.goto(BASE + "/allocate", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".budget", { timeout: 60000 });
  await page.waitForTimeout(400);

  const rows = await page.locator(".budget").count();
  if (rows === 0) fail("expected at least one allocate suggestion row");

  // No sub-line should be the old "Score NN%" duplicate, nor a standalone " · ".
  const subs = (await page.locator(".budget .budget-sub").allInnerTexts()).map((t) => t.trim());
  if (subs.some((t) => /^Score\b/.test(t))) fail(`a "Score …" duplicate sub-line is still rendered: ${JSON.stringify(subs)}`);
  if (subs.some((t) => t === "·" || t === "")) fail("a hand-rolled separator/empty sub-line is still present");
  if (!subs.some((t) => /^returns\b/.test(t))) fail(`the breakdown sub-line is missing (saw: ${JSON.stringify(subs)})`);

  // The score still appears once, in the row head.
  const head = (await page.locator(".budget .budget-amount").first().innerText()) || "";
  if (!/%/.test(head)) fail(`the head should still show the score percent, got "${head}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: allocate rows show the score once (head "${head.trim()}", breakdown sub-line only).`);
} finally {
  await browser.close();
}
