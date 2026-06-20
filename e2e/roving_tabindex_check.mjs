// L7 a11y gate - "radiogroups use roving tabindex". The ARIA radio pattern wants
// exactly ONE Tab stop per group (the checked option, tabindex=0) with the rest
// tabindex=-1, moved between by arrow keys. Checks the period Segmented and the
// accent SwatchPicker.
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

const scanGroups = (page) =>
  page.evaluate(() => {
    const out = [];
    document.querySelectorAll('[role="radiogroup"]').forEach((g, i) => {
      const radios = [...g.querySelectorAll('[role="radio"]')].filter((r) => r.offsetParent !== null);
      if (radios.length === 0) return;
      const tabStops = radios.filter((r) => r.getAttribute("tabindex") === "0").length;
      const checked = radios.find((r) => r.getAttribute("aria-checked") === "true");
      out.push({
        i,
        radios: radios.length,
        tabStops,
        hasChecked: !!checked,
        checkedIsTabStop: checked ? checked.getAttribute("tabindex") === "0" : null,
        label: g.getAttribute("aria-label") || "",
      });
    });
    return out;
  });

async function check(page, where) {
  const groups = await scanGroups(page);
  for (const g of groups) {
    if (g.tabStops !== 1) fail(`${where}: radiogroup "${g.label}" (#${g.i}) has ${g.tabStops} Tab stops across ${g.radios} radios — want exactly 1 (roving tabindex)`);
    if (g.hasChecked && !g.checkedIsTabStop) fail(`${where}: radiogroup "${g.label}" (#${g.i}) — the checked radio is not the Tab stop`);
  }
  return groups.length;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // /transactions carries the period Segmented (Week/Month/Quarter) in the top bar.
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(600);
  const n1 = await check(page, "/transactions");
  if (n1 === 0) fail("/transactions exposed no radiogroup to check (expected the period control)");

  // Settings exposes the accent SwatchPicker radiogroup.
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });
  await page.waitForTimeout(400);
  const n2 = await check(page, "Settings");
  if (n2 === 0) fail("Settings exposed no radiogroup (expected the accent swatches)");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: radiogroups use roving tabindex (${n1} on /transactions, ${n2} in Settings — exactly one Tab stop each).`);
} finally {
  await browser.close();
}
