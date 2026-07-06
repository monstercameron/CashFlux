// C69 gate — "a light theme lights the shell". ApplyTheme now derives the
// data-theme attribute (which drives the shell's light/dark skin) from the theme's
// own luminance (Theme.IsLight), instead of leaving it to the separate prefs system.
// A theme with a light background must set data-theme="light" on boot. Exits
// non-zero on any failure.
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

const dataTheme = (page) => page.evaluate(() => document.documentElement.getAttribute("data-theme"));

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // The default theme is dark, so the shell starts dark.
  const before = await dataTheme(page);
  if (before !== "dark") fail(`default data-theme = ${before}, want "dark"`);

  // Save a theme with a light background and reload — ApplyTheme runs after prefs at
  // boot, so it's the authoritative writer of data-theme.
  await page.evaluate(() =>
    localStorage.setItem("cashflux:theme", JSON.stringify({ name: "TestLight", bgBase: "#ffffff", bgCard: "#ffffff" })),
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  const after = await dataTheme(page);
  if (after !== "light") fail(`after a light theme, data-theme = ${after}, want "light" (shell would stay dark)`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: a light theme drives data-theme=light, so the shell skin follows the theme (C69).");
} finally {
  await browser.close();
}
