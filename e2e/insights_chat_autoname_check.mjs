// C82 gate — after a few exchanges the chat auto-generates a short title and updates
// its switcher tab. We answer normal turns with "ok" and the title request with a
// canned name, then assert the conversation pill shows that name. Exits non-zero on
// any failure.
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

const NAME = "Smart Money Plan";

try {
  const page = await browser.newPage();
  let namingCalls = 0;
  await page.route("**/chat/completions", async (route) => {
    const body = JSON.parse(route.request().postData() || "{}");
    const isNaming = (body.messages || []).some((m) => m.role === "system" && /short.*title/i.test(m.content));
    if (isNaming) namingCalls++;
    const content = isNaming ? NAME + "\n" : "ok";
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ choices: [{ finish_reason: "stop", message: { role: "assistant", content } }], usage: {} }) });
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

  // Two exchanges → 4 messages → auto-name fires.
  for (const msg of ["how is my spending", "what about savings"]) {
    await input.fill(msg);
    await card.getByRole("button", { name: "Send" }).first().click();
    await page.waitForTimeout(900);
  }

  // The naming request should fire and the switcher pill should show the name.
  await page.waitForFunction((n) => document.body.innerText.includes(n), NAME, { timeout: 8000 })
    .catch(() => fail("the chat tab was not renamed to the AI title"));
  if (namingCalls === 0) fail("the auto-name request was never made");

  // The name is on a switcher pill button (clickable tab), not just somewhere.
  const pill = page.getByRole("button", { name: NAME });
  if ((await pill.count()) === 0) fail("the AI title is not a switcher tab button");

  if (!process.exitCode) console.log(`PASS: chat auto-named to "${NAME}" (${namingCalls} naming call) and the tab updated.`);
} finally {
  await browser.close();
}
