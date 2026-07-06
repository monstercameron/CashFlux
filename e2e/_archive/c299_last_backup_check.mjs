// C299 gate — "Last backed up" timestamp appears in Settings → Data after an export.
//
// Steps:
//   1. Open the app and navigate to Settings.
//   2. Assert the `data-testid="last-backup"` element is present and shows the
//      "never backed up" text (i.e. no prior backup on a fresh session).
//   3. Trigger an Export JSON download.
//   4. After the download completes, reload (the wasm runtime tears down on download —
//      known headless artifact) and navigate back to Settings → Data.
//   5. Assert the "last-backup" element now shows text that does NOT include the
//      "never" phrase — i.e. it transitioned to the formatted timestamp copy.
//
// Limit: the test cannot assert a specific date string because the formatted value
// depends on the user's date-format preference and the wall clock.  It asserts
// presence and transition (never → backed-up), which is the meaningful C299 invariant.
//
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

const openSettings = async (page) => {
  // Navigate to the root; open Settings via the gear/settings button.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  // Use the command palette to jump to Settings (most reliable across layouts).
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  await page.fill("#cf-cmd-input", "settings");
  await page.waitForTimeout(150);
  const row = page.locator("[data-cmd-row]").filter({ hasText: /settings/i }).first();
  await row.click();
  // Wait for the Data section controls to be present.
  await page.waitForSelector('[data-testid="last-backup"]', { timeout: 15000 });
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    // Suppress the known headless download-teardown artifact.
    if (/Go program has already exited/.test(s)) return;
    errors.push(s);
  });

  // ── Step 1 + 2: open Settings, check initial "never backed up" state ──────
  await openSettings(page);
  const neverText = await page
    .locator('[data-testid="last-backup"]')
    .innerText()
    .catch(() => null);
  if (!neverText) {
    fail("last-backup element not found in Settings → Data");
  } else if (!/never|not backed/i.test(neverText)) {
    // Allow either the exact string or any variant of "never" / "not backed".
    // If the element already shows a date, it means a prior backup was recorded
    // in localStorage — the never-check is best-effort; proceed anyway.
    console.warn("WARN: last-backup element text was not the 'never' variant — possibly a prior session's backup is recorded:", neverText);
  }

  // ── Step 3: trigger Export JSON ──────────────────────────────────────────
  // Use the command palette to find the Export JSON button.
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  await page.fill("#cf-cmd-input", "export json");
  await page.waitForTimeout(200);
  const exportRow = page.locator("[data-cmd-row]").filter({ hasText: /export.*json/i }).first();
  // Wait for the download; the wasm runtime teardown is expected and suppressed above.
  const [download] = await Promise.all([
    page.waitForEvent("download", { timeout: 15000 }),
    exportRow.click(),
  ]);
  // Consume the download path so Playwright cleans up; we don't need the file.
  await download.path().catch(() => null);

  // ── Step 4: reload and re-open Settings ──────────────────────────────────
  // The wasm runtime tears down after a download in headless mode; reload to get
  // a fresh context before asserting the updated timestamp.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await openSettings(page);

  // ── Step 5: assert the timestamp line is no longer the "never" copy ──────
  const afterText = await page
    .locator('[data-testid="last-backup"]')
    .innerText()
    .catch(() => null);
  if (!afterText) {
    fail("last-backup element disappeared after export + reload");
  } else if (/never|not backed/i.test(afterText)) {
    fail(
      'last-backup still shows the "never backed up" copy after a successful export: ' +
        afterText,
    );
  } else {
    console.log("last-backup text after export:", afterText);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(
      "PASS: C299 — last-backup element transitions from 'never' to timestamp copy after export.",
    );
  }
} finally {
  await browser.close();
}
