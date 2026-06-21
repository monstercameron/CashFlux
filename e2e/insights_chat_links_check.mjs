// C90 gate — asset creation returns a deep link and the chat link navigates in-app
// (and dedupe blocks near-duplicates). add_task returns "[Open it](/todo#id)";
// clicking it routes to /todo and scrolls to the task. Creating a task that matches
// an existing one returns the existing item instead of cloning. Fresh page per run
// (one approval). Exits non-zero on any failure.
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

// run drives one add_task through approval; turn 2 echoes the tool result as the
// answer (so the deep link renders in the bubble). Returns { ctx, page, result }.
// linkMode: "relative" (/todo#id), "absolute" (same-origin URL), or "crosshost"
// (a different host — the model may fabricate one; an app-route link is still ours).
async function run(taskTitle, linkMode = "relative") {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  let result = "";
  await page.route("**/chat/completions", async (route) => {
    const body = JSON.parse(route.request().postData() || "{}");
    const toolMsgs = (body.messages || []).filter((m) => m.role === "tool");
    if (!toolMsgs.length) {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "tool_calls", message: { role: "assistant", content: "", tool_calls: [tc("t1", "add_task", { title: taskTitle, priority: "medium" })] } }], usage: {} }) });
    } else {
      result = toolMsgs.map((m) => m.content).join(" ");
      // Mirror how a real model may paraphrase the link's host.
      const host = linkMode === "absolute" ? BASE : linkMode === "crosshost" ? "http://localhost:65535" : "";
      const answer = host ? "Here you go — " + result.replace(/\]\(\/(todo[^)]*)\)/g, "](" + host + "/$1)") : "Here you go — " + result;
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content: answer } }], usage: {} }) });
    }
  });
  page.__reloads = 0;
  page.on("framenavigated", (f) => { if (f === page.mainFrame()) page.__reloads++; });
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
  await card.locator("#cf-chat-input").fill("add the task please");
  await card.getByRole("button", { name: "Send" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("wants to make a change"), { timeout: 8000 })
    .catch(() => fail("approval card did not appear"));
  await page.getByRole("button", { name: "Approve" }).first().click();
  await page.waitForFunction(() => document.body.innerText.includes("Here you go"), { timeout: 8000 })
    .catch(() => fail("no final answer"));
  return { ctx, page, result };
}

try {
  // 1) & 1b) New task → link returned + clicking it navigates IN-APP (no full reload)
  // to /todo and jumps to the row. Run once with a relative href and once with an
  // absolute same-origin href (how a real model may phrase it) — both must stay in-app.
  for (const linkMode of ["relative", "absolute", "crosshost"]) {
    const TITLE = "Research refinancing options e2e " + linkMode;
    const { ctx, page, result } = await run(TITLE, linkMode);
    const m = result.match(/\/todo#([^)\s]+)/);
    if (!m) fail("add_task result has no /todo#<id> link: " + JSON.stringify(result));
    const id = m && m[1];
    const link = page.locator(".insights-answer a", { hasText: "Open it" }).first();
    if ((await link.count()) === 0) fail("the answer did not render an 'Open it' link");
    if (linkMode !== "relative") {
      const href = await link.getAttribute("href");
      if (!/^https?:\/\//.test(href || "")) fail(`expected an absolute href for the ${linkMode} scenario, got: ` + href);
    }
    // A real full reload wipes this sentinel and re-boots the wasm (losing the in-memory task).
    await page.evaluate(() => { window.__cfSentinel = 0xC45F; });
    await link.click();
    await page.waitForFunction(() => location.pathname.replace(/\/$/, "").endsWith("/todo"), { timeout: 5000 })
      .catch(() => fail(`clicking the ${linkMode} link did not navigate to /todo`));
    if ((await page.evaluate(() => window.__cfSentinel)) !== 0xC45F) fail(`clicking the ${linkMode} link caused a FULL PAGE RELOAD`);
    if (!(await page.evaluate((t) => document.body.innerText.includes(t), TITLE))) fail("the task is not on the To-do screen after navigating (a reload would have wiped the in-memory store)");
    if (id) {
      await page.waitForSelector(`[id="${id}"]`, { timeout: 5000 })
        .catch(() => fail("the To-do row is missing the id anchor for jump-to-item"));
    }
    await ctx.close();
  }

  // 2) Dedupe — creating a task matching an existing sample task returns the existing one.
  {
    const { ctx, result } = await run("Pay credit card before the 22nd");
    if (!/already exists/i.test(result)) fail("dedupe did not catch a near-duplicate task: " + JSON.stringify(result));
    if (!/\/todo#/.test(result)) fail("the dedupe message did not link to the existing task: " + JSON.stringify(result));
    await ctx.close();
  }

  if (!process.exitCode) console.log("PASS: deep link navigates in-app with NO full reload (relative + absolute + cross-host hrefs); dedupe blocks near-duplicates.");
} finally {
  await browser.close();
}
