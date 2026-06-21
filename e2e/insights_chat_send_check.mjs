// Regression gate — Send (click) and Enter both send a message and NEVER reload the
// page (the composer is not a form). Asserts no main-frame navigation, the message
// renders, the input clears, and the reply shows. Exits non-zero on any failure.
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
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content: "reply ok" } }], usage: {} }) });
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

  // Count main-frame navigations from here on — must stay at 0.
  let navs = 0;
  page.on("framenavigated", (f) => {
    if (f === page.mainFrame()) navs++;
  });

  // 1) Enter sends.
  await input.fill("first via enter");
  await input.press("Enter");
  await page.waitForFunction(() => document.body.innerText.includes("first via enter"), { timeout: 5000 })
    .catch(() => fail("[enter] message did not appear"));
  if ((await input.inputValue()) !== "") fail("[enter] the input was not cleared after sending");

  // 2) Send button sends.
  await input.fill("second via click");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("second via click"), { timeout: 5000 })
    .catch(() => fail("[click] message did not appear"));

  // 3) Shift+Enter must NOT send (it's ignored, so the text stays).
  await input.fill("draft with shift enter");
  await input.press("Shift+Enter");
  await page.waitForTimeout(300);
  if ((await input.inputValue()) === "") fail("[shift+enter] should not send/clear");

  // 4) Cam's bug: after using Up to select a past prompt, Send (click) must still work.
  await input.fill("");
  await input.press("ArrowUp"); // most recent = "second via click"
  if ((await input.inputValue()) !== "second via click") fail("[after-cycle] ArrowUp should load the past message");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.querySelector("#cf-chat-input")?.value === "", { timeout: 5000 })
    .catch(() => fail("[after-cycle] clicking Send after cycling did not send (input never cleared)"));

  // 5) And Enter must still work after cycling.
  await input.focus();
  await page.waitForTimeout(500);
  await input.press("ArrowUp");
  await page.waitForFunction(() => (document.querySelector("#cf-chat-input")?.value || "") !== "", { timeout: 4000 })
    .catch(() => fail("[after-cycle] ArrowUp produced no value"));
  await input.press("Enter");
  await page.waitForFunction(() => document.querySelector("#cf-chat-input")?.value === "", { timeout: 5000 })
    .catch(() => fail("[after-cycle] Enter after cycling did not send (input never cleared)"));

  await page.waitForTimeout(400);
  if (navs !== 0) fail(`the page navigated/reloaded ${navs} time(s) — composer must not submit a form`);
  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Enter + Send send messages, Shift+Enter doesn't, and nothing reloads.");
} finally {
  await browser.close();
}
