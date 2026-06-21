// Blank Widget builder / Widget manager pages: both appear in the left rail and
// render their placeholder when navigated to. Exits non-zero on any failure.
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
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(600);

  const pages = [
    { label: "Widget builder", route: "/widget-builder", text: "Widget creation is coming soon." },
    { label: "Widget manager", route: "/widget-manager", text: "Widgets" }, // manager is live now
  ];
  for (const pg of pages) {
    const link = page.locator(`a[title="${pg.label}"]`).first();
    if ((await link.count()) === 0) { fail(`rail is missing the "${pg.label}" entry`); continue; }
    await link.click();
    await page.waitForFunction((r) => location.pathname.endsWith(r), pg.route, { timeout: 5000 })
      .catch(() => fail(`clicking "${pg.label}" did not navigate to ${pg.route}`));
    await page.waitForFunction((t) => document.body.innerText.includes(t), pg.text, { timeout: 5000 })
      .catch(() => fail(`"${pg.label}" page did not render its placeholder`));
  }

  if (!process.exitCode) console.log("PASS: Widget builder and Widget manager appear in the rail and render their blank pages.");
} finally {
  await browser.close();
}
