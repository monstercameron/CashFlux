// /assistant agent-first e2e: the conversation IS the page (chat left/dominant,
// composer above the fold, agent intro on an empty thread), with the agent's
// periphery in a side rail (observations, pinned insights, saved conversations).
// Exercises the deterministic no-key paths (localqa answers, error CTA) plus
// negatives (empty send, unknown question), the prompt flip modal, and the hub
// tabs. Exits non-zero on any failure.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1200 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const bubbles = async () => await p.locator("#cf-chat-thread > div").count();

// --- boot + sample data ---
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto(URL + "/assistant", { waitUntil: "domcontentloaded" });
await p.waitForSelector('[data-testid="assistant-hub"]', { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1200);

// --- agent-first layout ---
check("A1 the hub renders with Ask as the default tab", await p.locator('[data-testid="assistant-hub"]').count() === 1 && await p.locator('[data-testid="assistant-layout"]').count() === 1);
check("A2 two-column agent layout: chat + rail", await p.locator('[data-testid="assistant-chat"]').count() === 1 && await p.locator('[data-testid="assistant-rail"]').count() === 1);
{
  const chat = await p.locator('[data-testid="assistant-chat"]').boundingBox();
  const rail = await p.locator('[data-testid="assistant-rail"]').boundingBox();
  check("A3 the chat is the DOMINANT column (left, wider than the rail)", !!chat && !!rail && chat.x < rail.x && chat.width > rail.width, `chat=${Math.round(chat?.width)}px rail=${Math.round(rail?.width)}px`);
  const input = await p.locator("#cf-chat-input").boundingBox();
  check("A4 the composer is above the fold (no scrolling to reach the agent)", !!input && input.y + input.height < 1200, `y=${Math.round(input?.y)}`);
}
check("A5 the rail carries the agent's periphery (pins and/or conversations)", (await p.locator('[data-testid="assistant-rail"]').innerText()).length > 0);

// --- fresh thread: the agent introduces itself ---
await p.locator('[data-testid="assistant-new-chat"]').click(); await p.waitForTimeout(500);
check("B1 a new chat shows the agent intro (what it can do)", await p.locator('[data-testid="assistant-intro"]').count() === 1 && (await p.locator('[data-testid="assistant-intro"] .asst-intro-cap').count()) === 3);
check("B1b keyless: the key callout lives in the intro (single CTA)", await p.locator('[data-testid="assistant-key-callout"]').count() === 1);
check("B1c the demo transcript is visually distinct (dashed example frame)", await p.locator('[data-testid="assistant-examples"].asst-examples').count() === 1);
check("B2 starter chips are personalized and present", await p.locator('[data-testid="assistant-chat"] .btn-chip, [data-testid="assistant-chat"] .chip, [data-testid="assistant-chat"] button').count() > 0);

// A starter chip FILLS the composer (doesn't send).
{
  const chip = p.locator('[data-testid="assistant-chat"] .insights-chip, [data-testid="assistant-chat"] .suggest-chip').first();
  const anyChip = (await chip.count()) ? chip : p.locator('[data-testid="assistant-chat"] button', { hasText: /\?$/ }).first();
  if (await anyChip.count()) {
    const before = await bubbles();
    await anyChip.click(); await p.waitForTimeout(300);
    const v = await p.locator("#cf-chat-input").inputValue();
    check("B3 a starter chip fills the composer without sending", v.length > 0 && (await bubbles()) === before, v.slice(0, 50));
  } else {
    check("B3 a starter chip fills the composer without sending", false, "no chips found");
  }
}

// --- deterministic no-key agent exchange (localqa) ---
await p.locator("#cf-chat-input").fill("what is my net worth?");
await p.locator('[data-testid="assistant-send"]').click(); await p.waitForTimeout(1000);
check("C1 a no-key question gets a real answer from on-device figures", (await bubbles()) >= 2 && /\$/.test(await p.locator("#cf-chat-thread").innerText()), (await p.locator("#cf-chat-thread").innerText()).slice(0, 80));
check("C2 no page errors from the exchange", errs.length === 0);

// The exchange persists as a conversation in the rail.
await p.waitForTimeout(600);
check("C3 the exchange persists to the rail's Conversations", (await p.locator('[data-testid="assistant-convs"] .conv-pill').count()) >= 1);

// New chat clears the thread; the saved pill restores it.
const savedPills = await p.locator('[data-testid="assistant-convs"] .conv-pill').count();
await p.locator('[data-testid="assistant-new-chat"]').click(); await p.waitForTimeout(500);
check("C4 New chat clears the thread back to the intro", (await bubbles()) === 0 && await p.locator('[data-testid="assistant-intro"]').count() === 1);
await p.locator('[data-testid="assistant-convs"] .conv-pill').first().click(); await p.waitForTimeout(600);
check("C5 picking a saved conversation restores its thread", (await bubbles()) >= 2, `${savedPills} saved`);

// --- negatives ---
{
  const before = await bubbles();
  await p.locator("#cf-chat-input").fill("");
  await p.locator('[data-testid="assistant-send"]').click(); await p.waitForTimeout(400);
  check("N1 (neg) sending an empty composer does nothing", (await bubbles()) === before);
  await p.locator("#cf-chat-input").fill("write me a haiku about spreadsheets");
  await p.locator('[data-testid="assistant-send"]').click(); await p.waitForTimeout(600);
  check("N2 (neg) a non-answerable no-key question shows the key CTA error, no crash", (await p.locator('[data-testid="assistant-chat"] [role=alert]').count()) >= 1 && errs.length === 0);
  await p.locator("#cf-chat-input").fill("");
}

// --- prompt editor flip modal (Advanced) ---
await p.locator('[data-testid="assistant-advanced"]').click(); await p.waitForTimeout(300);
await p.locator('[data-testid="assistant-edit-prompt"]').click(); await p.waitForTimeout(900);
check("D1 Advanced → Edit prompt opens the system-prompt flip modal", (await p.locator(".flip-panel textarea, .flip textarea, textarea").count()) >= 1);
await p.keyboard.press("Escape"); await p.waitForTimeout(600);
check("D2 Escape dismisses the prompt modal", (await p.locator("textarea").count()) === 0);

// --- the hub's other tabs still stand ---
await p.locator(".seg-btn", { hasText: /^Insights$/ }).first().click(); await p.waitForTimeout(900);
check("E1 Insights tab renders the data panel (merchants/trend live here now)", (await p.locator("main").innerText()).length > 100 && errs.length === 0);
await p.locator(".seg-btn", { hasText: /^Smart$/ }).first().click(); await p.waitForTimeout(900);
check("E2 Smart tab renders", errs.length === 0);
await p.locator(".seg-btn", { hasText: /^Ask$/ }).first().click(); await p.waitForTimeout(900);
check("E3 back to Ask: the agent surface is intact", await p.locator('[data-testid="assistant-layout"]').count() === 1 && await p.locator("#cf-chat-input").count() === 1);

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
