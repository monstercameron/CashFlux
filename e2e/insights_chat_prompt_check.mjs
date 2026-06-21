// C82 gate — the user-editable system prompt. Open the chat's "Edit prompt" panel,
// set a custom persona, save it, then send a message and assert the request's leading
// system message is the custom prompt (and the live data-context message still
// follows it). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const MARKER = "CUSTOM_PERSONA_MARKER: speak like a pirate.";

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  let lastBody = null;
  await page.route("**/chat/completions", async (route) => {
    lastBody = JSON.parse(route.request().postData() || "{}");
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content: "Arr, here be yer answer." } }], usage: { total_tokens: 10 } }),
    });
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ rememberAiKey: true }));
    localStorage.setItem("cashflux:openai-key", "sk-test");
    localStorage.removeItem("cashflux:chat-system-prompt");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(600);
  await page.locator('a[title="Insights"]').first().click();
  await page.waitForTimeout(700);

  const card = page.locator(".card", { hasText: "Ask CashFlux" }).first();
  await card.waitFor({ timeout: 10000 });

  // Open the prompt editor, replace the persona, save.
  await card.getByRole("button", { name: "Edit prompt" }).first().click();
  const ta = page.locator("textarea").first();
  await ta.waitFor({ timeout: 5000 });
  const prefilled = await ta.inputValue();
  if (!/CashFlux/i.test(prefilled)) fail("the prompt editor did not prefill the current/default prompt");
  await ta.fill(MARKER);
  await page.locator(".set-btn.save").first().click();
  await page.waitForTimeout(300);

  // It should persist to localStorage.
  const stored = await page.evaluate(() => localStorage.getItem("cashflux:chat-system-prompt"));
  if (stored !== MARKER) fail("custom prompt was not persisted, got: " + JSON.stringify(stored));

  // Send a message; the request's first system message must be the custom prompt.
  await card.locator("input.field-wide").first().fill("hello");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("Arr, here be yer answer."), { timeout: 8000 })
    .catch(() => fail("no reply after sending with a custom prompt"));
  await page.waitForTimeout(200);

  const sys = (lastBody.messages || []).filter((m) => m.role === "system");
  if (!sys.length || sys[0].content !== MARKER) {
    fail("the custom prompt was not used as the leading system message: " + JSON.stringify(sys.map((s) => s.content.slice(0, 40))));
  }
  if (!sys.some((s) => /Live context/i.test(s.content))) {
    fail("the live data-context system message is missing (a custom prompt must not drop it)");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: editable system prompt persists and is used (with live context still appended).");
} finally {
  await browser.close();
}
