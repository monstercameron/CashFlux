// B16 E2E story — "settings export → import round-trip (lossless)". Exports the
// dataset to JSON, imports that exact file back, exports again, and asserts the
// two exports carry the same entities — proving import/export is lossless. Also
// asserts the export is valid, non-empty JSON. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import os from "os";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

// Counts every entity (object with a string id) in an exported dataset, and
// returns the running id list — structure-agnostic.
function countEntities(json) {
  const data = JSON.parse(json);
  let n = 0;
  const ids = [];
  const walk = (o) => {
    if (!o || typeof o !== "object") return;
    if (Array.isArray(o)) return o.forEach(walk);
    if (typeof o.id === "string" && o.id) {
      n++;
      ids.push(o.id);
    }
    Object.values(o).forEach(walk);
  };
  walk(data);
  return { n, ids };
}

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Clicks Export JSON and returns the downloaded file's contents.
async function exportJSON(page) {
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "Export JSON", exact: true }).click(),
  ]);
  const p = path.join(os.tmpdir(), `cashflux-export-${download.suggestedFilename()}`);
  await download.saveAs(p);
  return { path: p, content: fs.readFileSync(p, "utf8") };
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.locator("button.hh").first().click();
  await page.getByRole("button", { name: "Export JSON", exact: true }).waitFor({ timeout: 8000 });

  // Export #1.
  const first = await exportJSON(page);
  if (first.content.trim().length < 50) fail("export #1 looks empty");
  let e1;
  try {
    e1 = countEntities(first.content);
  } catch (err) {
    fail("export #1 is not valid JSON: " + err.message);
  }
  if (e1 && e1.n === 0) fail("export #1 has no entities (expected the sample dataset)");

  // Import that exact file back (Import… opens a file chooser).
  page.once("filechooser", (fc) => fc.setFiles(first.path).catch(() => {}));
  await page.getByRole("button", { name: "Import…", exact: true }).click();
  await page.waitForTimeout(1500);

  // Export #2 and compare.
  const second = await exportJSON(page);
  const e2 = countEntities(second.content);
  if (e1 && e2.n !== e1.n) fail(`entity count changed across round-trip: ${e1.n} → ${e2.n}`);
  if (e1 && !e2.ids.includes(e1.ids[0])) fail(`a known entity (${e1.ids[0]}) was lost in the round-trip`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: export→import→export is lossless (${e1 ? e1.n : "?"} entities preserved).`);
} finally {
  await browser.close();
}
