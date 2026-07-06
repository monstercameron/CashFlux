// C64 gate — "the rule author sees a live match count". Typing a match phrase in
// the add form should show how many existing transactions it would hit, before
// saving. Exits non-zero on any failure.
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

  await page.goto(BASE + "/rules", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#rule-add", { timeout: 60000 });

  // No preview before typing.
  const preview = () => page.locator('[role="status"]', { hasText: /Matches\s+\d+\s+transaction/ });
  if ((await preview().count()) !== 0) fail("a match-count preview showed before any phrase was typed");

  // A common substring should hit several sample transactions.
  await page.locator("#rule-add").fill("a");
  await page.waitForTimeout(400);
  if ((await preview().count()) === 0) fail('typing a phrase should show a live "Matches N transactions" preview');
  const txt = (await preview().first().innerText()).trim();
  const n = parseInt((txt.match(/Matches\s+(\d+)/) || [])[1] || "-1", 10);
  if (!(n > 0)) fail(`expected a positive live match count for "a", got "${txt}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: rule author sees a live match count ("${txt}").`);
} finally {
  await browser.close();
}
