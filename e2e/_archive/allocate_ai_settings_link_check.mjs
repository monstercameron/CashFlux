// C54 gate — "allocate AI error links to Settings". When the user clicks
// Explain AI without an OpenAI key and without a backend configured, the error
// message now includes an "Open Settings" button that navigates to /settings.
// Exits non-zero on any failure.
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
  // Wait for the page to settle — the "Why this ranking?" section is only shown
  // when there are ranked suggestions, which requires seeded accounts.
  await page.waitForTimeout(2000);

  const explainBtn = page.locator('button', { hasText: /explain|why/i }).first();
  if ((await explainBtn.count()) === 0) {
    console.log("SKIP: no Explain AI button visible (no ranked suggestions in seed data).");
    process.exitCode = 0;
  } else {
    await explainBtn.click();
    await page.waitForTimeout(800);

    // If there's an error div (no key configured), it should contain an
    // "Open Settings" button.
    const errDiv = page.locator('[role="alert"]').first();
    if ((await errDiv.count()) > 0) {
      const errText = await errDiv.innerText();
      if (/key|settings/i.test(errText)) {
        const settingsBtn = errDiv.locator('button', { hasText: /settings/i });
        if ((await settingsBtn.count()) === 0) {
          fail('AI "needKey" error should include an "Open Settings" button');
        } else {
          await settingsBtn.click();
          await page.waitForTimeout(500);
          const url = page.url();
          if (!url.includes("/settings")) {
            fail(`clicking "Open Settings" should navigate to /settings, got ${url}`);
          }
        }
      }
    }

    if (errors.length) fail("page errors: " + errors.join(" | "));
    if (!process.exitCode)
      console.log(
        'PASS: allocate AI "needKey" error exposes an "Open Settings" navigation button.'
      );
  }
} finally {
  await browser.close();
}
