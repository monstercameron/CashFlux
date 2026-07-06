// C274 gate — "single-device/local-first disclosure note on the Members screen".
// The Members screen must show an informational paragraph explaining that roles
// are organizational labels for a shared local dataset, and that there are no
// per-member logins or access controls.  Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const screenshotsDir = path.join(__dirname, "screenshots");
if (!fs.existsSync(screenshotsDir)) fs.mkdirSync(screenshotsDir, { recursive: true });

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  // Wait for the app to boot and render the Members screen.
  await page.waitForSelector('[data-testid="members-single-device-note"]', { timeout: 60000 });

  const note = page.locator('[data-testid="members-single-device-note"]');
  const count = await note.count();
  if (count === 0) {
    fail("disclosure note (data-testid=members-single-device-note) not found");
  } else {
    const text = (await note.first().innerText()).trim();

    // Verify key phrases are present.
    if (!text.includes("single shared dataset")) {
      fail(`note missing "single shared dataset" — got: "${text}"`);
    }
    if (!text.includes("no separate per-member logins")) {
      fail(`note missing "no separate per-member logins" — got: "${text}"`);
    }

    // Accessibility: the element must be a <p> (rendered as block text, not interactive).
    const tag = await note.first().evaluate((el) => el.tagName.toLowerCase());
    if (tag !== "p") {
      fail(`disclosure note should be a <p>, got <${tag}>`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  // Always take a screenshot for the record.
  await page.screenshot({ path: path.join(screenshotsDir, "c274_single_device_note.png"), fullPage: false });

  if (!process.exitCode) {
    console.log("PASS: single-device disclosure note is present, readable, and accessible on the Members screen.");
  }
} finally {
  await browser.close();
}
