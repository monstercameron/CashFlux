// Regression gate — a keydown event that lacks modifier properties (a plain Event,
// like a synthetic dispatch) must NOT crash the app. Before the fix the global
// shortcut listener called Value.Bool() on undefined and the whole Go program
// exited. Asserts no panic and the app stays alive (the chat still sends). Exits
// non-zero on any failure.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
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

  // Dispatch a keydown that has NO metaKey/ctrlKey/altKey (a bare Event). The old
  // code did Value.Bool() on these → panic → "Go program has already exited".
  await page.evaluate(() => {
    document.dispatchEvent(new Event("keydown", { bubbles: true }));
    const e2 = new Event("keydown", { bubbles: true });
    Object.defineProperty(e2, "code", { value: "KeyK" });
    document.dispatchEvent(e2);
  });
  await page.waitForTimeout(400);

  const fatal = errors.filter((e) => /Value\.Bool on undefined|Go program has already exited|exit code: 2/i.test(e));
  if (fatal.length) fail("a malformed keydown crashed the app: " + JSON.stringify(fatal));

  // The app must still be alive: navigate to Insights and send a message.
  await page.locator('a[title="Insights"]').first().click();
  await page.waitForTimeout(700);
  const card = page.locator(".card", { hasText: "Ask CashFlux" }).first();
  await card.waitFor({ timeout: 10000 });
  await page.locator("#cf-chat-input").fill("still alive?");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("still alive?"), { timeout: 5000 })
    .catch(() => fail("the app was not responsive after the malformed keydown"));

  if (errors.some((e) => /Go program has already exited/i.test(e))) fail("the Go program exited at some point: " + JSON.stringify(errors));
  if (!process.exitCode) console.log("PASS: malformed keydown does not crash the app; chat stays responsive.");
} finally {
  await browser.close();
}
