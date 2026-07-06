// GLAMOR G13 — Insights page visual + structural review for "The Money Question" (Renu).
// Reviews the AI chat / Q&A / no-key state, pinned insights, suggestion chips,
// composer placement, theming, and light-mode contrast.
// Takes screenshots at 1280, 1440, and 768 px in dark + light themes.
// Writes into e2e/screenshots/glamor_13_insights_*.png and glamor_13_insights_dom.json.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot = (name) => path.join(SHOTS, `glamor_13_insights_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

async function navToInsights(page) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(600);
  // Reset viewAsMember to Everyone
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try {
        const p = JSON.parse(raw);
        delete p.viewAsMember;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      } catch (_) {}
    }
  });
  // Ensure NO API key is set (no-key state)
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try {
        const p = JSON.parse(raw);
        // Also clear any stored settings that might have a key
        localStorage.removeItem("cashflux:settings-openai-key");
      } catch (_) {}
    }
  });
  const insightsLink = page.locator('nav a[title="Insights"]').first();
  await insightsLink.click();
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
}

// DOM audit for insights page structure
async function auditDOM(page) {
  return page.evaluate(() => {
    const cards = [...document.querySelectorAll(".card")];
    const cardTitles = cards.map(c => c.querySelector("h2,.card-title")?.textContent?.trim() || "(no title)");

    // Chat thread
    const threadEl = document.getElementById("cf-chat-thread");
    const hasThread = !!threadEl;
    const threadMsgCount = threadEl ? threadEl.querySelectorAll("[class*=rounded]").length : 0;

    // Composer / input
    const inputEl = document.getElementById("cf-chat-input");
    const hasInput = !!inputEl;
    const inputPlaceholder = inputEl ? inputEl.getAttribute("placeholder") : "";

    // No-key hint (muted paragraph + Settings button)
    const mutedEls = [...document.querySelectorAll(".muted")];
    const noKeyHintText = mutedEls.map(e => e.textContent?.trim()).find(t => t && (t.toLowerCase().includes("key") || t.toLowerCase().includes("api") || t.toLowerCase().includes("set up") || t.toLowerCase().includes("openai"))) || "";

    // Settings navigation button for no-key CTA
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim());
    const hasSettingsBtn = btnTexts.some(t => t.toLowerCase() === "settings" || t.toLowerCase().includes("settings"));
    const hasSendBtn = btnTexts.some(t => t.toLowerCase() === "send");
    const hasCancelBtn = btnTexts.some(t => t.toLowerCase() === "cancel");
    const hasNewChatBtn = btnTexts.some(t => t.toLowerCase().includes("new chat"));
    const hasEditPromptBtn = btnTexts.some(t => t.toLowerCase().includes("edit prompt") || t.toLowerCase().includes("persona"));

    // Suggestion chips
    const chipEls = [...document.querySelectorAll(".chip-suggest")];
    const chipCount = chipEls.length;
    const chipTexts = chipEls.map(c => c.textContent.trim()).slice(0, 5);

    // Pinned insights card
    const pinnedCard = cards.find(c => (c.querySelector("h2,.card-title")?.textContent || "").toLowerCase().includes("pinned"));
    const hasPinnedCard = !!pinnedCard;

    // Spending highlights card
    const highlightsCard = cards.find(c => (c.querySelector("h2,.card-title")?.textContent || "").toLowerCase().includes("highlight") || (c.querySelector("h2,.card-title")?.textContent || "").toLowerCase().includes("spending"));
    const hasHighlightsCard = !!highlightsCard;

    // Conversation switcher pills
    const convPills = [...document.querySelectorAll("[class*=rounded-full]")];
    const convPillCount = convPills.length;

    // Error message
    const errEl = document.querySelector(".err,[role=alert]");
    const errText = errEl ? errEl.textContent.trim() : "";

    // Layout
    const pageHeight = document.body.scrollHeight;
    const viewportH = window.innerHeight;
    const overflowCount = cards.filter(c => c.scrollWidth > c.clientWidth + 4).length;

    // Theming
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || cardTitleEl).backgroundColor : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";

    // User bubble / assistant bubble colors (not present in no-key state with no turns)
    const userBubble = document.querySelector('[class*="bg-sky"]');
    const userBubbleBg = userBubble ? getComputedStyle(userBubble).backgroundColor : "N/A";
    const assistBubble = document.querySelector('[class*="bg-black"]');
    const assistBubbleBg = assistBubble ? getComputedStyle(assistBubble).backgroundColor : "N/A";

    // Muted text contrast
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Composer is sticky / bottom positioned?
    const composerInput = document.getElementById("cf-chat-input");
    const composerParent = composerInput ? composerInput.closest("[class*=flex]") : null;
    const composerRect = composerParent ? composerParent.getBoundingClientRect() : null;
    const composerBottom = composerRect ? composerRect.bottom : -1;
    const composerNearBottom = composerBottom > 0 && composerBottom <= window.innerHeight + 20;

    return {
      cardTitles, hasThread, threadMsgCount, hasInput, inputPlaceholder,
      noKeyHintText, hasSettingsBtn, hasSendBtn, hasCancelBtn, hasNewChatBtn, hasEditPromptBtn,
      chipCount, chipTexts, hasPinnedCard, hasHighlightsCard, convPillCount, errText,
      pageHeight, viewportH, overflowCount,
      cardTitleColor, cardBg, pageBg, userBubbleBg, assistBubbleBg, mutedColor, dataTheme,
      composerNearBottom, btnTexts: btnTexts.slice(0, 20)
    };
  });
}

try {
  // ============================================================
  // DARK THEME SESSION
  // ============================================================
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await dark.setViewportSize({ width: 1280, height: 900 });
  await navToInsights(dark);

  // Screenshot: empty/no-key state at 1280 dark
  await dark.screenshot({ path: shot("1280_dark_nokey") });
  await dark.screenshot({ path: shot("1280_dark_nokey_full"), fullPage: true });

  // DOM audit
  const domAudit = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_13_insights_dom.json"), JSON.stringify(domAudit, null, 2));

  // Try typing in the chat input (no-key path — should show no-key error, not send)
  const inputEl = dark.locator("#cf-chat-input");
  if (await inputEl.count() > 0) {
    await inputEl.fill("Can I afford a $500 vacation?");
    await dark.waitForTimeout(400);
    await dark.screenshot({ path: shot("1280_dark_typed") });
    // Press Enter — should show no-key error or affordability fast-path result
    await dark.keyboard.press("Enter");
    await dark.waitForTimeout(1000);
    await dark.screenshot({ path: shot("1280_dark_after_send") });
  }

  // Screenshot at 1440
  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("1440_dark_nokey") });

  // Screenshot at 768
  await dark.setViewportSize({ width: 768, height: 1024 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("768_dark_nokey") });

  // ============================================================
  // LIGHT THEME SESSION
  // ============================================================
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));

  // Light theme recipe
  await light.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await light.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await light.waitForTimeout(400);
  await light.evaluate(() => localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" })));
  await light.reload();
  await light.waitForFunction(() => document.documentElement.getAttribute("data-theme") === "light");
  await light.waitForTimeout(600);

  await light.locator('nav a[title="Insights"]').first().click();
  await light.waitForSelector(".card", { timeout: 30000 });
  await light.waitForTimeout(1200);

  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("1280_light_nokey") });
  await light.screenshot({ path: shot("1280_light_nokey_full"), fullPage: true });

  // Type a question in light mode
  const lightInput = light.locator("#cf-chat-input");
  if (await lightInput.count() > 0) {
    await lightInput.fill("What did I spend on groceries?");
    await light.waitForTimeout(400);
    await light.screenshot({ path: shot("1280_light_typed") });
  }

  await light.setViewportSize({ width: 1440, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("1440_light_nokey") });

  await light.setViewportSize({ width: 768, height: 1024 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("768_light_nokey") });

  // Light contrast audit
  const lightContrast = await light.evaluate(() => {
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || cardTitleEl).backgroundColor : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;
    const mainEl = document.querySelector("main,.main-content,[class*=main]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";
    const inputEl = document.getElementById("cf-chat-input");
    const inputBg = inputEl ? getComputedStyle(inputEl).backgroundColor : "N/A";
    const inputColor = inputEl ? getComputedStyle(inputEl).color : "N/A";
    const chipEl = document.querySelector(".chip-suggest");
    const chipBg = chipEl ? getComputedStyle(chipEl).backgroundColor : "N/A";
    const chipColor = chipEl ? getComputedStyle(chipEl).color : "N/A";
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    // Check page background between cards (content area bg)
    const contentEl = document.querySelector(".content-area,.page-content,[data-page],main");
    const contentBg = contentEl ? getComputedStyle(contentEl).backgroundColor : "N/A";
    return { cardTitleColor, cardBg, pageBg, mainBg, mutedColor, inputBg, inputColor, chipBg, chipColor, contentBg, dataTheme };
  });

  fs.writeFileSync(path.join(SHOTS, "glamor_13_insights_light_contrast.json"), JSON.stringify(lightContrast, null, 2));

  // Persistence check: reload dark and confirm theme persists
  await dark.setViewportSize({ width: 1280, height: 900 });
  await dark.reload({ waitUntil: "domcontentloaded" });
  await dark.waitForTimeout(800);
  const themeAfterReload = await dark.evaluate(() => document.documentElement.getAttribute("data-theme"));

  // Summary output
  console.log("=== DOM Audit (dark/no-key) ===");
  console.log(JSON.stringify(domAudit, null, 2));
  console.log("=== Light Contrast ===");
  console.log(JSON.stringify(lightContrast, null, 2));
  console.log("theme after reload:", themeAfterReload);
  console.log("errors:", errors.length === 0 ? "none" : errors);
  console.log("shots dir:", SHOTS);

} finally {
  await browser.close();
}
