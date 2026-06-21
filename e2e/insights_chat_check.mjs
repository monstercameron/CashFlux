// C82 gate — the Insights chat actually talks to the OpenAI provider and renders
// the reply. We inject a remembered API key (so the composer shows) and intercept
// the OpenAI /chat/completions call with a canned Markdown answer, then drive a
// real send and assert: the user bubble appears, the request is made, the reply is
// marked-rendered, and Pin works. Exits non-zero on any failure.
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

  // Intercept the OpenAI chat-completions call. `mode` switches between a canned
  // success and a 401, so we can also assert errors surface in the UI.
  let aiCalls = 0;
  let lastBody = null;
  let mode = "ok";
  await page.route("**/chat/completions", async (route) => {
    aiCalls++;
    try {
      lastBody = JSON.parse(route.request().postData() || "{}");
    } catch {}
    if (mode === "401") {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: { message: "Invalid API key", type: "invalid_request_error" } }),
      });
      return;
    }
    // Simulate real OpenAI latency so any in-flight re-render race is exposed.
    await new Promise((r) => setTimeout(r, 1500));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        choices: [{ message: { role: "assistant", content: "## Spending\n\nYou spent **$420** on groceries last month." }, finish_reason: "stop" }],
        usage: { prompt_tokens: 50, completion_tokens: 12, total_tokens: 62 },
      }),
    });
  });

  // Boot once, then plant a remembered key + prefs and reload so hydrateAIKey loads it.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ rememberAiKey: true }));
    localStorage.setItem("cashflux:openai-key", "sk-test-e2e-key");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(500);

  // Navigate to Insights in-app (gwc dev has no deep-link fallback). The rail item
  // is an <a title="Insights"> with no href (the router handles the click).
  const navInsights = page.locator('a[title="Insights"]');
  if ((await navInsights.count()) === 0) fail("no Insights nav link in the rail");
  await navInsights.first().click();
  await page.waitForTimeout(600);

  const chatCard = page.locator(".card", { hasText: "Ask CashFlux" }).first();
  await chatCard.waitFor({ timeout: 10000 });

  // The composer input must be present (key is set).
  const input = chatCard.locator("input.field-wide").first();
  if ((await input.count()) === 0) fail("chat composer input not shown despite a key being set");
  await input.fill("How much did I spend on groceries?");

  // Send.
  const sendBtn = chatCard.getByRole("button", { name: "Send" }).first();
  if ((await sendBtn.count()) === 0) fail("no Send button in the composer");
  await sendBtn.click();

  // The user bubble should appear immediately.
  await page.waitForFunction(
    () => document.body.innerText.includes("How much did I spend on groceries?"),
    { timeout: 5000 }
  ).catch(() => fail("user message did not appear in the thread"));

  // The OpenAI provider must have been called, with our question in the body.
  await page.waitForFunction(() => true, { timeout: 100 });
  if (aiCalls === 0) fail("the chat never called the OpenAI /chat/completions endpoint");
  if (lastBody && JSON.stringify(lastBody).indexOf("groceries") === -1) {
    fail("the request body did not include the user's question");
  }

  // The assistant reply should render the canned Markdown via marked (an <h2> + <strong>).
  const answer = page.locator(".insights-answer").last();
  await answer.waitFor({ timeout: 8000 });
  await page.waitForTimeout(400);
  const html = await answer.innerHTML();
  if (!/<h2[\s>]/i.test(html)) fail("assistant reply was not marked-rendered (no <h2>): " + html.slice(0, 200));
  if (!/<strong>\$420<\/strong>/i.test(html)) fail("assistant reply missing the bold figure: " + html.slice(0, 200));

  // For a normal model the request carries temperature 0.4 (reasoning models omit it
  // via reasoningModel(); default model here is gpt-4o-mini).
  if (lastBody && lastBody.temperature !== 0.4) {
    fail("expected temperature 0.4 for a non-reasoning model, got " + JSON.stringify(lastBody.temperature));
  }

  // Pin the reply → a "Pinned insights" card appears above the chat.
  const pinBtn = page.getByRole("button", { name: "Pin" }).first();
  if ((await pinBtn.count()) > 0) {
    await pinBtn.click();
    await page.waitForTimeout(300);
    const pinnedCard = page.locator(".card", { hasText: "Pinned insights" });
    if ((await pinnedCard.count()) === 0) fail("Pin did not create a Pinned insights card");
  } else {
    fail("no Pin button on the assistant reply");
  }

  // A conversation pill should now exist in the switcher (autosaved).
  const pills = await page.getByRole("button", { name: /groceries/i }).count();
  if (pills === 0) fail("the conversation was not saved into the switcher");

  // --- Reproduce "open app, initial chat fails": reload so the saved conversation
  // is resumed by the init effect, then send the first message of the new session. ---
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.locator('a[title="Insights"]').first().click();
  await page.waitForTimeout(700);
  const resumedCard = page.locator(".card", { hasText: "Ask CashFlux" }).first();
  await resumedCard.waitFor({ timeout: 10000 });
  // The previous conversation should be resumed (its question is visible).
  const resumed = await page.evaluate(() => document.body.innerText.includes("How much did I spend on groceries?"));
  if (!resumed) fail("the saved conversation was not resumed on reload");

  const beforeCalls = aiCalls;
  await resumedCard.locator("input.field-wide").first().fill("And on dining?");
  await resumedCard.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(
    () => document.body.innerText.includes("And on dining?"),
    { timeout: 5000 }
  ).catch(() => fail("[resume] user message did not appear"));
  // The key assertion: the first send of a resumed session must reach OpenAI and reply.
  await page.waitForFunction(
    (n) => window.__noassert || document.querySelectorAll(".insights-answer").length >= 2,
    {},
    {}
  ).catch(() => {});
  await page.waitForTimeout(2500);
  if (aiCalls <= beforeCalls) fail("[resume] the initial chat after reload did NOT call OpenAI (this is the bug)");
  const answers = await page.locator(".insights-answer").count();
  if (answers < 2) fail(`[resume] expected a new assistant reply after reload, got ${answers} answer bubbles`);

  // Error path: a rejected key must surface a visible error, not fail silently.
  mode = "401";
  await page.getByRole("button", { name: "New chat" }).first().click();
  await page.waitForTimeout(200);
  const input2 = chatCard.locator("input.field-wide").first();
  await input2.fill("anything");
  await chatCard.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(
    () => !!document.querySelector(".err") && /key|openai/i.test(document.querySelector(".err").innerText),
    { timeout: 6000 }
  ).catch(() => fail("a 401 from OpenAI did not surface a visible error in the chat"));

  await page.screenshot({ path: path.join(__dirname, "insights-chat.png") });

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: chat called OpenAI (${aiCalls}x), rendered Markdown, pinned + autosaved, temp=0.4, and a 401 shows a visible error.`);
} finally {
  await browser.close();
}
