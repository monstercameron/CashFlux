// L12 regression gate - "the boot splash fully dismisses, on every route". The
// #boot overlay used to linger (translucent, position:fixed, z-index:10) over the
// app on /planning, /split, /goals, /documents. It must be gone (display:none or
// faded out) shortly after the app renders.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const ROUTES = ["/planning", "/split", "/goals", "/documents"];

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const bootState = (page) =>
  page.evaluate(() => {
    const b = document.getElementById("boot");
    if (!b) return { gone: true };
    const cs = getComputedStyle(b);
    return {
      gone: cs.display === "none" || Number(cs.opacity) === 0 || b.classList.contains("hidden"),
      display: cs.display,
      opacity: cs.opacity,
      pointer: cs.pointerEvents,
    };
  });

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  for (const route of ROUTES) {
    await page.goto(BASE + route, { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app *", { timeout: 60000 }); // app rendered
    await page.waitForTimeout(1500); // allow the fade + display:none removal
    const s = await bootState(page);
    if (!s.gone) {
      fail(`boot splash still visible on ${route}: ${JSON.stringify(s)}`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: boot splash fully dismisses on ${ROUTES.join(", ")}.`);
} finally {
  await browser.close();
}
