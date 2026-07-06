// C90 gate — mutating tools require approval, then actually change the data.
// Each scenario runs on a fresh page (a single tool run): Approve → the task is
// created and shows on To-do; Decline → the tool is stopped and nothing is created.
// Exits non-zero on any failure.
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
const tc = (id, n, a) => ({ id, type: "function", function: { name: n, arguments: JSON.stringify(a) } });

// scenario opens a fresh page, drives one add_task tool through the approval card
// (clicking `decision`), and returns the captured tool results + whether the task
// landed on the To-do screen.
async function scenario(decision, taskTitle) {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const toolResults = [];
  let title = taskTitle;
  await page.route("**/chat/completions", async (route) => {
    const body = JSON.parse(route.request().postData() || "{}");
    const toolMsgs = (body.messages || []).filter((m) => m.role === "tool");
    if (toolMsgs.length) toolResults.push(...toolMsgs.map((m) => m.content));
    if (!toolMsgs.length) {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "tool_calls", message: { role: "assistant", content: "", tool_calls: [tc("t1", "add_task", { title })] } }], usage: {} }) });
    } else {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content: "Done handled it" } }], usage: {} }) });
    }
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
  await card.locator("input.field-wide").first().fill("please add the task");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("wants to make a change"), { timeout: 8000 })
    .catch(() => fail(`[${decision}] approval card did not appear`));
  await page.getByRole("button", { name: decision }).first().click();
  await page.waitForTimeout(3000);
  const finalShown = await page.evaluate(() => !!document.querySelector("#cf-chat-thread")?.innerText.includes("Done handled it"));
  await page.locator('a[title="To-do"]').first().click();
  await page.waitForTimeout(800);
  const onTodo = await page.evaluate((t) => document.body.innerText.includes(t), taskTitle);
  await ctx.close();
  return { toolResults, finalShown, onTodo };
}

try {
  // Approve path.
  const ap = await scenario("Approve", "Approve me oat milk e2e");
  if (!ap.finalShown) fail("[approve] no final answer after approving");
  if (!ap.toolResults.some((c) => /Added to-do:/i.test(c))) fail("[approve] add_task did not run: " + JSON.stringify(ap.toolResults));
  if (!ap.onTodo) fail("[approve] the approved task is not on the To-do screen");

  // Decline path.
  const dc = await scenario("Decline", "Decline me e2e never exists");
  if (!dc.finalShown) fail("[decline] no final answer after declining");
  if (!dc.toolResults.some((c) => /declined/i.test(c))) fail("[decline] the tool was not stopped: " + JSON.stringify(dc.toolResults));
  if (dc.onTodo) fail("[decline] a declined task was still created");

  if (!process.exitCode) console.log("PASS: approve → task created + on To-do; decline → tool stopped, no change.");
} finally {
  await browser.close();
}
