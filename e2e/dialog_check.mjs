// C42 — in-app modal dialogs replace native prompt/confirm. The "New page" rail
// action opens a prompt modal; confirming creates+navigates, cancelling does not.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
try {
  const page = await (await browser.newContext()).newPage();
  // A native prompt/confirm would hang headless; fail loudly if one ever opens.
  page.on("dialog", async (d) => { fail("a NATIVE dialog opened: " + d.type()); await d.dismiss(); });
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title], .bento', { timeout: 60000 });
  await page.waitForTimeout(700);

  // Open the "New page" prompt → an in-app modal with a text input appears.
  await page.getByText("New page", { exact: true }).first().click();
  await page.waitForSelector(".cf-dialog .cf-dialog-input", { timeout: 8000 }).catch(() => fail("prompt modal did not open"));

  // Cancel → modal closes, no navigation.
  await page.locator(".cf-dialog button", { hasText: "Cancel" }).first().click();
  await page.waitForTimeout(300);
  if ((await page.locator(".cf-dialog").count()) !== 0) fail("Cancel did not close the dialog");

  // Reopen, fill, confirm → navigates to the new /p/<slug> page.
  await page.getByText("New page", { exact: true }).first().click();
  await page.waitForSelector(".cf-dialog-input", { timeout: 8000 });
  await page.locator(".cf-dialog-input").fill("E2E Dialog Page");
  await page.locator("#cf-dialog-confirm").click();
  await page.waitForFunction(() => location.pathname.includes("/p/"), { timeout: 6000 })
    .catch(() => fail("confirming the prompt did not create + navigate to the page"));

  if (!process.exitCode) console.log("PASS: in-app prompt modal opens, cancels cleanly, and confirms to create the page (no native dialogs).");
} finally {
  await browser.close();
}
