// C82 gate — the Insights chat's TOOLS actually run against the user's live data.
// We inject a key, intercept OpenAI, and on the first turn return a batch of
// tool_calls (one per tool). The wasm tool loop executes them locally against the
// sample dataset and sends the results back; we assert each tool's result appears in
// the follow-up request body, then that the final answer renders. Exits non-zero on
// any failure.
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
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  const bodies = [];
  let turn = 0;
  await page.route("**/chat/completions", async (route) => {
    bodies.push(JSON.parse(route.request().postData() || "{}"));
    turn++;
    if (turn === 1) {
      // First turn: ask to run every tool at once.
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          choices: [{ finish_reason: "tool_calls", message: { role: "assistant", content: "", tool_calls: [
            toolCall("c1", "spending_by_category", { category: "Groceries", period: "all" }),
            toolCall("c2", "calculator", { expression: "income - spending" }),
            toolCall("c3", "list_members", {}),
            toolCall("c4", "account_balances", {}),
            toolCall("c5", "list_transactions", { category: "Dining", limit: 5 }),
            toolCall("c6", "financial_summary", {}),
            toolCall("c7", "list_budgets", {}),
            toolCall("c8", "list_goals", {}),
            toolCall("c9", "list_tasks", {}),
            toolCall("c10", "list_recurring", {}),
            toolCall("c11", "spending_breakdown", { period: "this_month" }),
          ] } }],
          usage: { total_tokens: 40 },
        }),
      });
      return;
    }
    // Second turn: the request now carries the tool results — answer.
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        choices: [{ finish_reason: "stop", message: { role: "assistant", content: "**Here's what I found** across your finances." } }],
        usage: { total_tokens: 80 },
      }),
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
  await card.locator("input.field-wide").first().fill("Give me a full rundown of my finances.");
  await card.getByRole("button", { name: "Send" }).first().click();

  // Wait for the loop to finish (final answer rendered).
  await page.waitForFunction(
    () => document.body.innerText.includes("Here's what I found"),
    { timeout: 12000 }
  ).catch(() => fail("the tool loop never produced a final answer"));
  await page.waitForTimeout(300);

  // Two model turns: tools requested, then answered.
  if (turn < 2) fail(`expected 2 model turns (tools + answer), got ${turn}`);

  // First request must have advertised the tools.
  if (!bodies[0] || !Array.isArray(bodies[0].tools) || bodies[0].tools.length < 6) {
    fail("first request did not advertise the tool set: " + JSON.stringify(bodies[0] && bodies[0].tools && bodies[0].tools.length));
  }

  // The follow-up request carries one tool-result message per call; check each ran
  // against real data.
  const toolMsgs = (bodies[1].messages || []).filter((m) => m.role === "tool");
  const byCall = Object.fromEntries(toolMsgs.map((m) => [m.tool_call_id, m.content]));
  const check = (id, re, label) => {
    if (!byCall[id]) fail(`no tool result for ${label} (${id})`);
    else if (!re.test(byCall[id])) fail(`${label} result looks wrong: ${JSON.stringify(byCall[id])}`);
  };
  check("c1", /Spent .* on Groceries \(\d+ transactions/i, "spending_by_category");
  check("c2", /income - spending = -?[\d.]+/i, "calculator");
  check("c3", /Members:/i, "list_members");
  check("c4", /.+: .*\$/s, "account_balances");
  check("c5", /\[.+\]|No matching transactions/i, "list_transactions");
  check("c6", /Net worth .* savings rate/i, "financial_summary");
  check("c7", /limit|No budgets/i, "list_budgets");
  check("c8", /%|No savings goals/i, "list_goals");
  check("c9", /\[.+\].+priority|No tasks/i, "list_tasks");
  check("c10", /next |No recurring/i, "list_recurring");
  check("c11", /Top spending|No spending/i, "spending_breakdown");

  // Spot-check the numbers are real (groceries had transactions in the sample).
  if (byCall.c1 && /Spent \$0\.00 on Groceries \(0 transactions/i.test(byCall.c1)) {
    fail("spending_by_category returned zero — it didn't see the sample transactions");
  }

  await page.screenshot({ path: path.join(__dirname, "insights-chat-tools.png") });
  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(`PASS: tool loop ran ${Object.keys(byCall).length} tools against live data:`);
    for (const [k, v] of Object.entries(byCall)) console.log(`  ${k}: ${v.replace(/\n/g, " | ").slice(0, 90)}`);
  }
} finally {
  await browser.close();
}
