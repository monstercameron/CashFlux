// UX gate — Up/Down in the Insights composer cycles the user's previous messages
// (shell-style), with the draft restored after cycling past the newest. Exits
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

try {
  const page = await browser.newPage();
  await page.route("**/chat/completions", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content: "ok" } }], usage: {} }) });
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ rememberAiKey: true }));
    localStorage.setItem("cashflux:openai-key", "sk-test");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.locator('a[title="Insights"]').first().click();
  await page.waitForTimeout(700);
  const card = page.locator(".card", { hasText: "Ask CashFlux" }).first();
  await card.waitFor({ timeout: 10000 });
  const input = page.locator("#cf-chat-input");

  // Send two messages.
  for (const msg of ["first message", "second message"]) {
    await input.fill(msg);
    await card.getByRole("button", { name: "Send" }).first().click();
    await page.waitForTimeout(700);
  }

  const valAfter = async () => input.inputValue();
  await input.focus();

  // Type a draft, then cycle.
  await input.fill("draft text");
  await input.press("ArrowUp");
  await page.waitForTimeout(120);
  if ((await valAfter()) !== "second message") fail("ArrowUp #1 should show the newest message, got: " + (await valAfter()));
  await input.press("ArrowUp");
  await page.waitForTimeout(120);
  if ((await valAfter()) !== "first message") fail("ArrowUp #2 should show the older message, got: " + (await valAfter()));
  await input.press("ArrowUp");
  await page.waitForTimeout(120); // clamp at oldest
  if ((await valAfter()) !== "first message") fail("ArrowUp at oldest should clamp, got: " + (await valAfter()));
  await input.press("ArrowDown");
  await page.waitForTimeout(120);
  if ((await valAfter()) !== "second message") fail("ArrowDown should go to the newer message, got: " + (await valAfter()));
  await input.press("ArrowDown");
  await page.waitForTimeout(120); // past newest → restore draft
  if ((await valAfter()) !== "draft text") fail("ArrowDown past newest should restore the draft, got: " + (await valAfter()));

  if (!process.exitCode) console.log("PASS: Up/Down cycles previous messages and restores the draft.");
} finally {
  await browser.close();
}
