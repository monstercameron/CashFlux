// C59 gate — "the no-key AI hint links to Settings". With no OpenAI key set, the
// Insights screen used to show a dead-end hint; it now offers a button that hops to
// Settings. Exits non-zero on any failure.
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

  await page.goto(BASE + "/insights", { waitUntil: "domcontentloaded" });
  // The "Explain my month" card carries the no-key hint + Settings button.
  const card = page.locator(".card", { hasText: "Add your OpenAI key" }).first();
  await card.waitFor({ timeout: 60000 });

  const btn = card.getByRole("button", { name: "Settings" });
  if ((await btn.count()) === 0) fail("the no-key hint has no Settings button");
  await btn.first().click();

  await page.waitForFunction(() => location.pathname.endsWith("/settings"), { timeout: 5000 }).catch(() => fail("the hint button did not navigate to /settings"));

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Insights no-key hint links to Settings.");
} finally {
  await browser.close();
}
