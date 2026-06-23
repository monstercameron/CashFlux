// C80 gate — "the product version is visible at the rail foot". Exits non-zero on
// any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { ready } from "./_ready.mjs";

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

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await ready(page);
  await page.waitForSelector(".app-version", { timeout: 10000 });

  const v = (await page.locator(".app-version").first().innerText()).trim();
  if (!/^v\d+\.\d+\.\d+/.test(v)) fail(`rail version should read like "v0.1.0", got "${v}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: rail foot shows the product version ("${v}").`);
} finally {
  await browser.close();
}
