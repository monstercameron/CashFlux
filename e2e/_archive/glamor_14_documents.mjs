// GLAMOR G14 — Documents page visual + structural review for "Import the Statement" (Omar).
// Reviews the image import zone, bank-statement paste area, CSV import, draft review table,
// dedupe, account selector, theming, and light-mode contrast.
// Takes screenshots at 1280, 1440, and 768 px in dark + light themes.
// Pastes a tiny sample CSV to exercise the review state.
// Writes into e2e/screenshots/glamor_14_documents_*.png and glamor_14_documents_dom.json.
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

const shot = (name) => path.join(SHOTS, `glamor_14_documents_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// Tiny sample bank statement for the review state — uses the statement parse path
// (auto-detected columns, no AI required).
const SAMPLE_STATEMENT = `Date,Description,Amount
2026-06-01,Salary ACH,4200.00
2026-06-02,Whole Foods,-86.40
2026-06-03,Netflix Subscription,-15.99
2026-06-04,Gas Station,-48.20
2026-06-05,Transfer In,500.00`;

async function navToDocuments(page) {
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
  const docsLink = page.locator('nav a[title="Documents"]').first();
  await docsLink.click();
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
}

// DOM audit for documents page structure
async function auditDOM(page) {
  return page.evaluate(() => {
    const cards = [...document.querySelectorAll(".card")];
    const cardTitles = cards.map(c => c.querySelector("h2,.card-title")?.textContent?.trim() || "(no title)");

    // Import entry zones
    const chooseImageBtn = [...document.querySelectorAll("button")].find(b => b.textContent.trim().toLowerCase().includes("choose image"));
    const hasChooseImageBtn = !!chooseImageBtn;
    const readAIBtn = [...document.querySelectorAll("button")].find(b => b.textContent.trim().toLowerCase().includes("read with ai") || b.textContent.trim().toLowerCase().includes("read"));
    const hasReadAIBtn = !!readAIBtn;

    // Statement paste area
    const textareas = [...document.querySelectorAll("textarea")];
    const hasStmtTextarea = textareas.some(t => t.placeholder && t.placeholder.toLowerCase().includes("posting date") || t.placeholder && t.placeholder.toLowerCase().includes("date"));
    const textareaCount = textareas.length;

    // CSV import area
    const hasCSVTextarea = textareas.some(t => t.placeholder && (t.placeholder.toLowerCase().includes("date,payee") || t.placeholder.toLowerCase().includes("csv")));

    // Account selector(s)
    const selects = [...document.querySelectorAll("select")];
    const acctSelects = selects.filter(s => s.getAttribute("aria-label") && s.getAttribute("aria-label").toLowerCase().includes("account"));
    const hasAccountSelect = acctSelects.length > 0;

    // Import / Parse buttons
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim());
    const hasImportBtn = btnTexts.some(t => t.toLowerCase() === "import" || t.toLowerCase().includes("import these"));
    const hasParseBtn = btnTexts.some(t => t.toLowerCase().includes("parse statement"));

    // Review table (draft rows) — only present after parse
    const draftRows = [...document.querySelectorAll(".rows .row")];
    const hasDraftRows = draftRows.length > 0;
    const draftRowCount = draftRows.length;

    // History card
    const historyCard = cards.find(c => (c.querySelector("h2,.card-title")?.textContent || "").toLowerCase().includes("history"));
    const hasHistoryCard = !!historyCard;
    const historyEmpty = historyCard ? !!historyCard.querySelector(".empty") : false;

    // Image preview (C60 says none by default — only shown after choosing an image)
    const imagePreview = document.querySelector('[data-testid="doc-image-preview"]');
    const hasImagePreview = !!imagePreview;

    // Errors
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

    // Muted text
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Field background (input/textarea)
    const fieldEl = document.querySelector(".field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    return {
      cardTitles, cardCount: cards.length,
      hasChooseImageBtn, hasReadAIBtn,
      hasStmtTextarea, hasCSVTextarea, textareaCount,
      hasAccountSelect,
      hasImportBtn, hasParseBtn,
      hasDraftRows, draftRowCount,
      hasHistoryCard, historyEmpty,
      hasImagePreview,
      errText, btnTexts: btnTexts.slice(0, 20),
      pageHeight, viewportH, overflowCount,
      cardTitleColor, cardBg, pageBg, mutedColor, fieldBg, fieldColor, dataTheme
    };
  });
}

try {
  // ============================================================
  // DARK THEME SESSION — empty state
  // ============================================================
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await dark.setViewportSize({ width: 1280, height: 900 });
  await navToDocuments(dark);

  // Screenshot: empty state at 1280 dark
  await dark.screenshot({ path: shot("dark_1280_empty") });
  await dark.screenshot({ path: shot("dark_1280_empty_full"), fullPage: true });

  // DOM audit (empty state)
  const domAudit = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_14_documents_dom.json"), JSON.stringify(domAudit, null, 2));

  // Screenshot at 1440
  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_1440_empty") });

  // Screenshot at 768
  await dark.setViewportSize({ width: 768, height: 1024 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_768_empty") });

  // ============================================================
  // DARK THEME — paste sample statement → review state
  // ============================================================
  await dark.setViewportSize({ width: 1280, height: 900 });
  await dark.waitForTimeout(300);

  // Find the statement textarea (first textarea — it's the statement paste area)
  const stmtTextareas = await dark.locator("textarea").all();
  let stmtPasted = false;
  if (stmtTextareas.length > 0) {
    // The statement textarea placeholder includes "Posting Date" — find it
    for (const ta of stmtTextareas) {
      const placeholder = await ta.getAttribute("placeholder");
      if (placeholder && (placeholder.includes("Posting Date") || placeholder.includes("Date") || placeholder.includes("date"))) {
        await ta.fill(SAMPLE_STATEMENT);
        stmtPasted = true;
        break;
      }
    }
    // Fallback: use the first textarea
    if (!stmtPasted && stmtTextareas.length > 0) {
      await stmtTextareas[0].fill(SAMPLE_STATEMENT);
      stmtPasted = true;
    }
  }

  await dark.waitForTimeout(400);

  if (stmtPasted) {
    // Click "Parse statement" button
    const parseBtn = dark.locator('button:has-text("Parse statement")').first();
    if (await parseBtn.count() > 0) {
      await parseBtn.click();
      await dark.waitForTimeout(1200);

      // Screenshot: review state (draft rows visible)
      await dark.screenshot({ path: shot("dark_1280_review") });
      await dark.screenshot({ path: shot("dark_1280_review_full"), fullPage: true });

      // DOM audit after parse
      const domAuditReview = await auditDOM(dark);
      fs.writeFileSync(path.join(SHOTS, "glamor_14_documents_dom_review.json"), JSON.stringify(domAuditReview, null, 2));
    }
  }

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

  await light.locator('nav a[title="Documents"]').first().click();
  await light.waitForSelector(".card", { timeout: 30000 });
  await light.waitForTimeout(1200);

  // Screenshots at 1280, 1440, 768 light — empty state
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_1280_empty") });
  await light.screenshot({ path: shot("light_1280_empty_full"), fullPage: true });

  await light.setViewportSize({ width: 1440, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("light_1440_empty") });

  await light.setViewportSize({ width: 768, height: 1024 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("light_768_empty") });

  // Light — paste statement → review state
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(300);

  const lightTextareas = await light.locator("textarea").all();
  let lightPasted = false;
  for (const ta of lightTextareas) {
    const placeholder = await ta.getAttribute("placeholder");
    if (placeholder && (placeholder.includes("Posting Date") || placeholder.includes("Date") || placeholder.includes("date"))) {
      await ta.fill(SAMPLE_STATEMENT);
      lightPasted = true;
      break;
    }
  }
  if (!lightPasted && lightTextareas.length > 0) {
    await lightTextareas[0].fill(SAMPLE_STATEMENT);
    lightPasted = true;
  }

  if (lightPasted) {
    const lightParseBtn = light.locator('button:has-text("Parse statement")').first();
    if (await lightParseBtn.count() > 0) {
      await lightParseBtn.click();
      await light.waitForTimeout(1200);
      await light.screenshot({ path: shot("light_1280_review") });
      await light.screenshot({ path: shot("light_1280_review_full"), fullPage: true });
    }
  }

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
    const fieldEl = document.querySelector(".field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";
    const textareaEl = document.querySelector("textarea.field");
    const textareaBg = textareaEl ? getComputedStyle(textareaEl).backgroundColor : "N/A";
    const textareaColor = textareaEl ? getComputedStyle(textareaEl).color : "N/A";
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    // Check page background between cards (content area bg)
    const contentEl = document.querySelector(".content-area,.page-content,[data-page],main");
    const contentBg = contentEl ? getComputedStyle(contentEl).backgroundColor : "N/A";
    // Draft row colors (if review state)
    const draftRowEl = document.querySelector(".rows .row");
    const draftRowBg = draftRowEl ? getComputedStyle(draftRowEl).backgroundColor : "N/A";
    const draftRowColor = draftRowEl ? getComputedStyle(draftRowEl).color : "N/A";
    // Amount text color
    const amtEl = document.querySelector(".amount.fig");
    const amtColor = amtEl ? getComputedStyle(amtEl).color : "N/A";
    return {
      cardTitleColor, cardBg, pageBg, mainBg, mutedColor,
      fieldBg, fieldColor, textareaBg, textareaColor,
      contentBg, draftRowBg, draftRowColor, amtColor, dataTheme
    };
  });

  fs.writeFileSync(path.join(SHOTS, "glamor_14_documents_light_contrast.json"), JSON.stringify(lightContrast, null, 2));

  // Summary output
  console.log("=== DOM Audit (dark/empty) ===");
  console.log(JSON.stringify(domAudit, null, 2));
  console.log("=== Light Contrast ===");
  console.log(JSON.stringify(lightContrast, null, 2));
  console.log("errors:", errors.length === 0 ? "none" : errors);
  console.log("shots dir:", SHOTS);

} finally {
  await browser.close();
}
