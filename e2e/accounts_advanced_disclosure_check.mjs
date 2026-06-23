// C49 gate — "asset advanced fields sit behind an 'Advanced' disclosure". Asserts
// the asset add-form hides the optional scoring fields (expected return, liquidity,
// stability, lock-until) until the disclosure toggle is expanded, keeping the
// common path short. Exits non-zero on any failure.
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

const advCount = async (page) => {
  const texts = await page.locator(".labeled-field span").allInnerTexts();
  // Field labels: "Return %", "Easy to access (1–5)" (or "Liquidity (1–5)"),
  // "Low risk (1–5)" (or "Stability (1–5)"), and "Locked until ..."
  const candidates = [
    "Return %",
    "Easy to access (1–5)", "Liquidity (1–5)",
    "Low risk (1–5)", "Stability (1–5)",
    "Locked until (no new money before this date)",
  ];
  const normalized = texts.map((t) => t.trim());
  const found = new Set();
  for (const c of candidates) {
    if (normalized.some((t) => t === c)) {
      // map to canonical bucket
      if (c.startsWith("Easy to access") || c.startsWith("Liquidity")) found.add("liquidity");
      else if (c.startsWith("Low risk") || c.startsWith("Stability")) found.add("stability");
      else if (c === "Return %") found.add("return");
      else if (c.startsWith("Locked")) found.add("locked");
    }
  }
  return found.size;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Open the add modal so the disclosure toggle is accessible.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForTimeout(400);
  await page.waitForSelector(".cf-adv-toggle", { timeout: 10000 });

  const toggle = page.locator(".cf-adv-toggle").first();
  if ((await toggle.getAttribute("aria-expanded")) !== "false") fail("toggle should start collapsed (aria-expanded=false)");

  // Collapsed: none of the advanced labels are present.
  let n = await advCount(page);
  if (n !== 0) fail(`advanced fields should be hidden when collapsed, saw ${n}`);

  await toggle.click();
  await page.waitForTimeout(300);

  if ((await page.locator(".cf-adv-toggle").first().getAttribute("aria-expanded")) !== "true") fail("toggle should be expanded after click");
  n = await advCount(page);
  if (n !== 4) fail(`expected all 4 advanced fields after expand, saw ${n}`);

  // Collapses again.
  await page.locator(".cf-adv-toggle").first().click();
  await page.waitForTimeout(300);
  n = await advCount(page);
  if (n !== 0) fail(`advanced fields should hide again on re-collapse, saw ${n}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: asset advanced fields sit behind a working disclosure toggle.");
} finally {
  await browser.close();
}
