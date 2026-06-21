// C82 gate — the chat can web_search AND calculate. The model asks to web_search tax
// brackets and run the calculator; the wasm loop performs a real fetch (mocked) and
// the formula calc, feeds both back, and answers. Asserts both tool results land in
// the follow-up request. Exits non-zero on any failure.
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
const toolCall = (id, name, args) => ({ id, type: "function", function: { name, arguments: JSON.stringify(args) } });

try {
  // Block the service worker so page.route can intercept the (otherwise SW-originated)
  // cross-origin DuckDuckGo fetch.
  const context = await browser.newContext({ serviceWorkers: "block" });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Mock DuckDuckGo Instant Answer.
  let ddgCalls = 0;
  await page.route(/duckduckgo\.com/, async (route) => {
    ddgCalls++;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ AbstractText: "For 2025, U.S. federal income tax brackets are 10%, 12%, 22%, 24%, 32%, 35%, and 37%. Florida has no state income tax.", Answer: "", Definition: "", RelatedTopics: [] }),
    });
  });

  const bodies = [];
  let turn = 0;
  await page.route("**/chat/completions", async (route) => {
    bodies.push(JSON.parse(route.request().postData() || "{}"));
    turn++;
    if (turn === 1) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ choices: [{ finish_reason: "tool_calls", message: { role: "assistant", content: "", tool_calls: [
          toolCall("w1", "web_search", { query: "2025 federal income tax brackets" }),
          toolCall("w2", "calculator", { expression: "income * 12" }),
        ] } }], usage: { total_tokens: 30 } }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content: "Based on the brackets and your annualized income, here's an estimate." } }], usage: { total_tokens: 60 } }),
    });
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ rememberAiKey: true }));
    localStorage.setItem("cashflux:openai-key", "sk-test");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(600);
  await page.locator('a[title="Insights"]').first().click();
  await page.waitForTimeout(700);

  const card = page.locator(".card", { hasText: "Ask CashFlux" }).first();
  await card.waitFor({ timeout: 10000 });
  await card.locator("input.field-wide").first().fill("How much do I pay in taxes in Florida based on my income?");
  await card.getByRole("button", { name: "Send" }).first().click();

  await page.waitForFunction(() => document.body.innerText.includes("here's an estimate"), { timeout: 12000 })
    .catch(() => fail("the loop never produced a final answer"));
  await page.waitForTimeout(300);

  if (ddgCalls === 0) fail("web_search did not actually fetch the search endpoint");
  if (turn < 2) fail(`expected 2 model turns, got ${turn}`);

  const toolMsgs = (bodies[1].messages || []).filter((m) => m.role === "tool");
  const byCall = Object.fromEntries(toolMsgs.map((m) => [m.tool_call_id, m.content]));
  if (!byCall.w1 || !/37%/.test(byCall.w1)) fail("web_search result missing the searched facts: " + JSON.stringify(byCall.w1));
  if (!byCall.w2 || !/income \* 12 = [\d.]+/.test(byCall.w2)) fail("calculator result looks wrong: " + JSON.stringify(byCall.w2));

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: web_search fetched (${ddgCalls}x) → "${byCall.w1.slice(0, 70)}…"; calculator → "${byCall.w2}".`);
} finally {
  await browser.close();
}
