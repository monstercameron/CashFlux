// Verifies the per-font remove UI (B20): upload a dummy font, confirm its row +
// Remove button appear, click Remove, and assert the font is gone from the store,
// the @font-face is cleared, and the active theme falls back off the removed
// family. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const FAMILY = "RemoveMe";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });

  page.on("filechooser", async (fc) => {
    await fc.setFiles({ name: "RemoveMe.woff2", mimeType: "font/woff2", buffer: Buffer.alloc(128, 9) });
  });
  await page.getByRole("button", { name: "Upload font" }).click();
  await page.waitForTimeout(400);

  // Row + remove control present; font stored and selected.
  const removeBtn = page.getByRole("button", { name: `Remove ${FAMILY}`, exact: true });
  if ((await removeBtn.count()) === 0) fail(`expected a "Remove ${FAMILY}" button after upload`);
  const afterAdd = await page.evaluate(() => ({
    stored: localStorage.getItem("cashflux:fonts") || "",
    fontUi: (JSON.parse(localStorage.getItem("cashflux:theme") || "{}")).fontUi,
  }));
  if (!afterAdd.stored.includes(FAMILY)) fail(`cashflux:fonts should contain ${FAMILY}`);
  if (afterAdd.fontUi !== FAMILY) fail(`theme.fontUi should be ${FAMILY} after upload, got ${afterAdd.fontUi}`);

  // Remove it.
  await removeBtn.click();
  await page.waitForTimeout(400);
  const afterRemove = await page.evaluate((fam) => ({
    stored: localStorage.getItem("cashflux:fonts") || "[]",
    fontUi: (JSON.parse(localStorage.getItem("cashflux:theme") || "{}")).fontUi,
    faceCss: document.getElementById("cashflux-fonts")?.textContent || "",
    rowGone: !document.querySelector(`button[aria-label="Remove ${fam}"]`),
  }), FAMILY);
  if (afterRemove.stored.includes(FAMILY)) fail(`cashflux:fonts should no longer contain ${FAMILY}, got ${afterRemove.stored}`);
  if (afterRemove.faceCss.includes(FAMILY)) fail(`@font-face for ${FAMILY} should be removed`);
  if (afterRemove.fontUi !== "Inter") fail(`theme.fontUi should fall back to Inter, got ${afterRemove.fontUi}`);
  if (!afterRemove.rowGone) fail("the font row should disappear after removal");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: uploaded font can be removed; store + @font-face cleared, theme falls back to Inter.");
} finally {
  await browser.close();
}
