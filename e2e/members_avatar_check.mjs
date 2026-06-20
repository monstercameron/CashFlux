// C62 gate — "members show a colored initial avatar". Each member row now renders
// a disc with the member's first initial, tinted with their color. Exits non-zero
// on any failure.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".member-avatar", { timeout: 60000 });

  const av = page.locator(".member-avatar").first();
  const initial = (await av.innerText()).trim();
  if (!/^[A-Z?]$/.test(initial)) fail(`avatar initial should be a single uppercase letter, got "${initial}"`);

  // It renders as a round, background-tinted disc (computed style).
  const style = await av.evaluate((el) => {
    const cs = getComputedStyle(el);
    return { radius: cs.borderRadius, bg: cs.backgroundColor, w: cs.width };
  });
  if (!/50%|9999px|999px/.test(style.radius)) fail(`avatar should be round, border-radius="${style.radius}"`);
  if (!style.bg || style.bg === "rgba(0, 0, 0, 0)") fail(`avatar should have a background color, got "${style.bg}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: member avatar renders ("${initial}" on a ${style.bg} disc).`);
} finally {
  await browser.close();
}
