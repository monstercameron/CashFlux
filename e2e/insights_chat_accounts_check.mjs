// C90 gate — account/transfer write tools. add_account creates a liability (e.g. a
// 401(k) loan) that shows on the Accounts screen; add_transfer moves money between
// accounts and reports it. Each runs on a fresh page (one approval). Exits non-zero
// on any failure.
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
const tc = (id, n, a) => ({ id, type: "function", function: { name: n, arguments: JSON.stringify(a) } });

// run drives one tool call through the approval card and returns the captured tool
// result + the page (for further checks). Caller closes the context.
async function run(toolName, args) {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const toolResults = [];
  await page.route("**/chat/completions", async (route) => {
    const body = JSON.parse(route.request().postData() || "{}");
    const toolMsgs = (body.messages || []).filter((m) => m.role === "tool");
    if (toolMsgs.length) toolResults.push(...toolMsgs.map((m) => m.content));
    if (!toolMsgs.length) {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "tool_calls", message: { role: "assistant", content: "", tool_calls: [tc("x1", toolName, args)] } }], usage: {} }) });
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
  await card.locator("#cf-chat-input").fill("please do it");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("wants to make a change"), { timeout: 8000 })
    .catch(() => fail(`[${toolName}] approval card did not appear`));
  await page.getByRole("button", { name: "Approve" }).first().click();
  await page.waitForTimeout(2500);
  return { ctx, page, toolResults };
}

try {
  // add_account → a liability shows on the Accounts screen.
  {
    const LOAN = "E2E 401k Loan";
    const { ctx, page, toolResults } = await run("add_account", { name: LOAN, class: "liability", balance: 10000, type: "loan", apr: 5 });
    if (!toolResults.some((c) => /Created liability account/i.test(c))) fail("add_account did not create the liability: " + JSON.stringify(toolResults));
    await page.locator('a[title="Accounts"]').first().click();
    await page.waitForTimeout(900);
    const body = await page.evaluate(() => document.body.innerText);
    if (!body.includes(LOAN)) fail("the new liability account is not on the Accounts screen");
    await ctx.close();
  }

  // add_transfer → money moves and it's reported.
  {
    const { ctx, toolResults } = await run("add_transfer", { from_account: "401(k)", to_account: "Cash Wallet", amount: 10000 });
    if (!toolResults.some((c) => /Transferred .* from .* to /i.test(c))) fail("add_transfer did not report a transfer: " + JSON.stringify(toolResults));
    await ctx.close();
  }

  if (!process.exitCode) console.log("PASS: add_account created a liability (on Accounts); add_transfer moved money.");
} finally {
  await browser.close();
}
