// GLAMOR G15 — Customize page visual + structural review for "The Formula Tinkerer" (Tomás).
// Reviews the formula builder (input, example buttons, save/load, result display),
// the available-variables reference (click-insert, formatting), the saved formulas card,
// the CustomFieldsManager section, two-tool layout, light-mode contrast, and 768px behaviour.
// Screenshots at 1280 / 1440 / 768 × dark + light.
// Writes into e2e/screenshots/glamor_15_customize_*.png and glamor_15_customize_dom.json.
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

const shot = (name) => path.join(SHOTS, `glamor_15_customize_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------
async function navToCustomize(page) {
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
  // Navigate via nav link
  const link = page.locator('nav a[title="Customize"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    // fallback: try any nav link whose text includes Customize
    const fallback = page.locator('nav a').filter({ hasText: /customize/i }).first();
    if (await fallback.count() > 0) await fallback.click();
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1000);
}

// ---------------------------------------------------------------
// DOM audit
// ---------------------------------------------------------------
async function auditDOM(page) {
  return page.evaluate(() => {
    const cards = [...document.querySelectorAll(".card")];
    const cardTitles = cards.map(c => {
      const t = c.querySelector("h2,.card-title");
      return t ? t.textContent.trim() : "(no title)";
    });

    // Formula builder inputs
    const formulaInput = document.querySelector('input[class*="field"]');
    const hasFormulaInput = !!formulaInput;
    const formulaInputType = formulaInput ? formulaInput.type : "N/A";
    const formulaPlaceholder = formulaInput ? formulaInput.placeholder : "N/A";

    // Formula name input
    const allInputs = [...document.querySelectorAll('input[class*="field"]')];
    const nameInput = allInputs.length > 1 ? allInputs[1] : null;
    const hasNameInput = !!nameInput;

    // Example / try buttons
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim());
    const exampleBtns = btns.filter(b => {
      const cls = b.className || "";
      return cls.includes("data-btn");
    });
    const exampleBtnCount = exampleBtns.length;
    const exampleBtnLabels = exampleBtns.map(b => b.textContent.trim());

    // Save button
    const hasSaveBtn = btnTexts.some(t => t.toLowerCase() === "save" || t.toLowerCase().includes("save formula"));

    // Result display
    const resultCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && t.textContent.toLowerCase().includes("result");
    });
    const hasResultCard = !!resultCard;
    const resultStatValue = resultCard ? resultCard.querySelector(".stat-value") : null;
    const hasResultValue = !!resultStatValue;
    const resultValueText = resultStatValue ? resultStatValue.textContent.trim() : "N/A";
    const resultHint = resultCard ? resultCard.querySelector(".muted") : null;
    const resultHintText = resultHint ? resultHint.textContent.trim() : "N/A";

    // Saved formulas card
    const savedCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && t.textContent.toLowerCase().includes("saved");
    });
    const hasSavedCard = !!savedCard;
    const savedRows = savedCard ? [...savedCard.querySelectorAll(".row")] : [];
    const savedRowCount = savedRows.length;

    // Variables card
    const varsCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && (t.textContent.toLowerCase().includes("variable") || t.textContent.toLowerCase().includes("available"));
    });
    const hasVarsCard = !!varsCard;
    const varRows = varsCard ? [...varsCard.querySelectorAll(".row")] : [];
    const varCount = varRows.length;
    const varNames = varRows.map(r => r.querySelector(".row-desc")?.textContent?.trim() || "");
    const varValues = varRows.map(r => r.querySelector(".amount")?.textContent?.trim() || "");

    // Custom fields manager — look for a card above the formula calculator
    const customFieldsCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && (t.textContent.toLowerCase().includes("custom field") || t.textContent.toLowerCase().includes("field"));
    });
    const hasCustomFieldsCard = !!customFieldsCard;
    const customFieldsTitle = customFieldsCard ? customFieldsCard.querySelector("h2,.card-title")?.textContent?.trim() : "N/A";
    const cfSelects = customFieldsCard ? [...customFieldsCard.querySelectorAll("select")] : [];
    const cfSelectCount = cfSelects.length;
    const cfInputs = customFieldsCard ? [...customFieldsCard.querySelectorAll('input[class*="field"]')] : [];
    const cfInputCount = cfInputs.length;
    const cfBtns = customFieldsCard ? [...customFieldsCard.querySelectorAll("button")].map(b => b.textContent.trim()) : [];

    // Layout: card order (by DOM position)
    const cardOrder = cardTitles;

    // Overflow
    const overflowCount = cards.filter(c => c.scrollWidth > c.clientWidth + 4).length;
    const pageHeight = document.body.scrollHeight;
    const viewportH = window.innerHeight;

    // Theming
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || document.body).backgroundColor : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;

    // Var row text colors (for contrast checks)
    const varRowEl = varRows[0] || null;
    const varLabelColor = varRowEl ? getComputedStyle(varRowEl.querySelector(".row-desc") || varRowEl).color : "N/A";
    const varValueColor = varRowEl ? getComputedStyle(varRowEl.querySelector(".amount") || varRowEl).color : "N/A";

    // Result value color
    const resultValEl = document.querySelector(".stat-value");
    const resultValColor = resultValEl ? getComputedStyle(resultValEl).color : "N/A";

    // Muted
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Field background
    const fieldEl = document.querySelector("input.field,textarea.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    // Check for visible labels (not just placeholder)
    const allLabels = [...document.querySelectorAll("label")];
    const labelTexts = allLabels.map(l => l.textContent.trim());

    // Page errors
    const errEl = document.querySelector(".err,[role=alert]");
    const errText = errEl ? errEl.textContent.trim() : "";

    return {
      cardTitles, cardCount: cards.length, cardOrder,
      hasFormulaInput, formulaInputType, formulaPlaceholder,
      hasNameInput,
      exampleBtnCount, exampleBtnLabels, hasSaveBtn,
      hasResultCard, hasResultValue, resultValueText, resultHintText,
      hasSavedCard, savedRowCount,
      hasVarsCard, varCount, varNames, varValues,
      hasCustomFieldsCard, customFieldsTitle, cfSelectCount, cfInputCount, cfBtns,
      overflowCount, pageHeight, viewportH, dataTheme,
      cardTitleColor, cardBg, pageBg,
      varLabelColor, varValueColor, resultValColor, mutedColor,
      fieldBg, fieldColor,
      labelTexts,
      errText, btnTexts: btnTexts.slice(0, 30),
    };
  });
}

// ---------------------------------------------------------------
// Light contrast audit
// ---------------------------------------------------------------
async function auditLightContrast(page) {
  return page.evaluate(() => {
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const pageBg = getComputedStyle(document.body).backgroundColor;

    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || document.body).backgroundColor : "N/A";

    // Var rows
    const varRowEl = document.querySelector('.rows .row .row-desc');
    const varLabelColor = varRowEl ? getComputedStyle(varRowEl).color : "N/A";
    const varRowBg = varRowEl ? getComputedStyle(varRowEl.closest(".row") || document.body).backgroundColor : "N/A";

    const varAmtEl = document.querySelector('.rows .row .amount');
    const varAmtColor = varAmtEl ? getComputedStyle(varAmtEl).color : "N/A";

    // Stat value (result)
    const statEl = document.querySelector(".stat-value");
    const statColor = statEl ? getComputedStyle(statEl).color : "N/A";
    const statBg = statEl ? getComputedStyle(statEl.closest(".card") || document.body).backgroundColor : "N/A";

    // Muted text
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Field
    const fieldEl = document.querySelector("input.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    // Saved formula row
    const savedRowEl = document.querySelector(".rows .row .row-meta");
    const savedMetaColor = savedRowEl ? getComputedStyle(savedRowEl).color : "N/A";

    // Body bg between cards
    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Example/data-btn buttons
    const dataBtnEl = document.querySelector(".data-btn");
    const dataBtnColor = dataBtnEl ? getComputedStyle(dataBtnEl).color : "N/A";
    const dataBtnBg = dataBtnEl ? getComputedStyle(dataBtnEl).backgroundColor : "N/A";

    return {
      dataTheme, pageBg,
      cardTitleColor, cardBg,
      varLabelColor, varRowBg, varAmtColor,
      statColor, statBg,
      mutedColor,
      fieldBg, fieldColor,
      savedMetaColor,
      mainBg,
      dataBtnColor, dataBtnBg,
    };
  });
}

// ---------------------------------------------------------------
// Exercise the formula builder
// ---------------------------------------------------------------
async function exerciseFormulaBuilder(page) {
  // Type a formula in the expression input (first .field input)
  const inputs = await page.locator('input.field').all();
  if (inputs.length > 0) {
    await inputs[0].fill("round((income - expense) / income * 100)");
    await page.waitForTimeout(600);
  }
}

try {
  // ============================================================
  // DARK THEME SESSION
  // ============================================================
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await dark.setViewportSize({ width: 1280, height: 900 });
  await navToCustomize(dark);

  // Screenshot: empty formula state at 1280 dark
  await dark.screenshot({ path: shot("dark_1280_empty") });
  await dark.screenshot({ path: shot("dark_1280_empty_full"), fullPage: true });

  // DOM audit (empty state)
  const domAudit = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_15_customize_dom.json"), JSON.stringify(domAudit, null, 2));

  // Exercise formula builder: type an expression, check result card
  await exerciseFormulaBuilder(dark);
  await dark.screenshot({ path: shot("dark_1280_formula") });

  // Click first example button
  const exampleBtns = dark.locator(".data-btn");
  if (await exampleBtns.count() > 0) {
    await exampleBtns.first().click();
    await dark.waitForTimeout(500);
    await dark.screenshot({ path: shot("dark_1280_example") });
  }

  // DOM audit with formula active
  const domAuditFormula = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_15_customize_dom_formula.json"), JSON.stringify(domAuditFormula, null, 2));

  // Screenshot at 1440
  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_1440") });

  // Screenshot at 768
  await dark.setViewportSize({ width: 768, height: 1024 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_768") });
  await dark.screenshot({ path: shot("dark_768_full"), fullPage: true });

  // ============================================================
  // LIGHT THEME SESSION
  // ============================================================
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));

  // Light theme recipe (G4 canonical)
  await light.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await light.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await light.waitForTimeout(400);
  await light.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    const p = raw ? JSON.parse(raw) : {};
    p.theme = "light";
    localStorage.setItem("cashflux:prefs", JSON.stringify(p));
  });
  await light.reload({ waitUntil: "domcontentloaded" });
  await light.waitForFunction(() => document.documentElement.getAttribute("data-theme") === "light", { timeout: 30000 });
  await light.waitForTimeout(600);
  console.log("[ok] theme 'light' confirmed on <html>");

  // Navigate to Customize in light mode
  const lightLink = light.locator('nav a[title="Customize"]').first();
  if (await lightLink.count() > 0) {
    await lightLink.click();
  } else {
    const fb = light.locator('nav a').filter({ hasText: /customize/i }).first();
    if (await fb.count() > 0) await fb.click();
  }
  await light.waitForSelector(".card", { timeout: 30000 });
  await light.waitForTimeout(1000);

  // Screenshots at 1280 light
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_1280_empty") });
  await light.screenshot({ path: shot("light_1280_empty_full"), fullPage: true });

  // Exercise formula builder in light
  await exerciseFormulaBuilder(light);
  await light.waitForTimeout(500);
  await light.screenshot({ path: shot("light_1280_formula") });

  // Click first example button in light
  const lightExBtns = light.locator(".data-btn");
  if (await lightExBtns.count() > 0) {
    await lightExBtns.first().click();
    await light.waitForTimeout(500);
    await light.screenshot({ path: shot("light_1280_example") });
  }

  // At 1440 light
  await light.setViewportSize({ width: 1440, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("light_1440") });

  // At 768 light
  await light.setViewportSize({ width: 768, height: 1024 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("light_768") });
  await light.screenshot({ path: shot("light_768_full"), fullPage: true });

  // Light contrast audit
  const lightContrast = await auditLightContrast(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_15_customize_light_contrast.json"), JSON.stringify(lightContrast, null, 2));
  console.log("[light contrast]", JSON.stringify(lightContrast, null, 2));

  // ============================================================
  // Summarise
  // ============================================================
  console.log("\n--- GLAMOR G15 CUSTOMIZE DOM AUDIT (empty/dark) ---");
  console.log(JSON.stringify(domAudit, null, 2));
  console.log("\n--- GLAMOR G15 CUSTOMIZE DOM AUDIT (formula/dark) ---");
  console.log(JSON.stringify(domAuditFormula, null, 2));

  if (errors.length > 0) {
    console.error("\n[PAGE ERRORS]", errors);
    process.exit(1);
  }

  const shots = [
    "dark_1280_empty", "dark_1280_empty_full",
    "dark_1280_formula", "dark_1280_example",
    "dark_1440", "dark_768", "dark_768_full",
    "light_1280_empty", "light_1280_empty_full",
    "light_1280_formula", "light_1280_example",
    "light_1440", "light_768", "light_768_full",
  ];
  console.log("\n[screenshots produced]");
  for (const s of shots) {
    const p = shot(s);
    console.log(" ", fs.existsSync(p) ? "✓" : "✗", path.basename(p));
  }

  console.log("\n[ok] G15 Customize review complete. Exit 0.");
} catch (err) {
  console.error("[FATAL]", err);
  process.exit(1);
} finally {
  await browser.close();
}
