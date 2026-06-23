// GLAMOR G18 — Rules page visual + structural review for "Set It and Forget It" (Bianca).
// Reviews the rule list (RuleRow: grip, match, meta, shadow-warning, match-count),
// the add-rule builder form (condition+action+tags), the precedence/reorder affordance (C64),
// the Mermaid precedence chain (C64), the suggestions card, shadow-warning display, and
// light-mode contrast. Screenshots at 1280 / 1440 / 768 × dark + light.
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

const shot = (name) => path.join(SHOTS, `glamor_18_rules_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------
async function navToRules(page) {
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
  const link = page.locator('nav a[title="Rules"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /rules/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/rules", { waitUntil: "domcontentloaded" });
  }
  await page.waitForTimeout(1200);
}

// Apply light theme
async function applyLight(page) {
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({theme:'light'})));
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 15000 });
  await page.waitForTimeout(800);
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

    // Rule rows
    const rows = [...document.querySelectorAll(".rows .row")];
    const rowCount = rows.length;

    // Rule grip handles (drag-to-reorder, C64)
    const grips = [...document.querySelectorAll(".rule-grip")];
    const gripCount = grips.length;
    const gripTitles = grips.map(g => g.getAttribute("title") || g.textContent.trim());

    // Warning/shadow spans in rows
    const warnEls = [...document.querySelectorAll(".row-meta")].filter(el => {
      return el.className.includes("warn") || el.textContent.includes("shadow") || el.textContent.includes("never");
    });
    const warnCount = warnEls.length;
    const warnTexts = warnEls.map(el => el.textContent.trim()).slice(0, 5);

    // Mermaid precedence chain (C64)
    const mermaidEls = [...document.querySelectorAll(".mermaid,svg[id*='mermaid'],figure[aria-label]")];
    const hasMermaid = mermaidEls.length > 0;
    const mermaidAriaLabel = mermaidEls[0]?.getAttribute("aria-label") || mermaidEls[0]?.closest("[aria-label]")?.getAttribute("aria-label") || "N/A";

    // Suggestions card
    const suggestCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && (t.textContent.toLowerCase().includes("suggest") || t.textContent.toLowerCase().includes("suggestion"));
    });
    const hasSuggestCard = !!suggestCard;
    const suggestTitle = suggestCard ? suggestCard.querySelector("h2,.card-title")?.textContent?.trim() : "N/A";
    const suggestRows = suggestCard ? [...suggestCard.querySelectorAll(".row")] : [];
    const suggestRowCount = suggestRows.length;
    const suggestBtns = suggestRows.map(r => r.querySelector("button")?.textContent?.trim() || "");

    // Rule list card
    const listCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && (t.textContent.toLowerCase().includes("rule") || t.textContent.toLowerCase().includes("auto"));
    });
    const listCardTitle = listCard ? listCard.querySelector("h2,.card-title")?.textContent?.trim() : "N/A";

    // Apply to existing button
    const applyBtn = [...document.querySelectorAll("button")].find(b =>
      b.textContent.toLowerCase().includes("apply") && b.textContent.toLowerCase().includes("existing")
    );
    const hasApplyBtn = !!applyBtn;
    const applyBtnClass = applyBtn ? applyBtn.className : "N/A";
    const applyBtnText = applyBtn ? applyBtn.textContent.trim() : "N/A";

    // Coverage text (rules.coverage)
    const muteds = [...document.querySelectorAll(".muted")];
    const coverageText = muteds.find(m => m.textContent.includes("transaction"))?.textContent?.trim() || "N/A";

    // Edit/Delete buttons on rule rows
    const editBtns = [...document.querySelectorAll("button")].filter(b =>
      b.textContent.toLowerCase() === "edit" || b.title?.toLowerCase() === "edit rule"
    );
    const editBtnCount = editBtns.length;

    // Is the add form visible on this page (or does user need to click to open)?
    const addForm = document.querySelector("form,.form-grid");
    const hasAddForm = !!addForm;
    const addFormEls = addForm ? [...addForm.querySelectorAll("input,select")] : [];
    const addFormFieldCount = addFormEls.length;

    // Overflow
    const overflowCount = cards.filter(c => c.scrollWidth > c.clientWidth + 4).length;

    // Theming
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || document.body).backgroundColor : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;

    // Row text colors
    const rowDescEl = document.querySelector(".rows .row .row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowMetaEl = document.querySelector(".rows .row .row-meta");
    const rowMetaColor = rowMetaEl ? getComputedStyle(rowMetaEl).color : "N/A";

    // Muted
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Labels
    const allLabels = [...document.querySelectorAll("label")];
    const labelTexts = allLabels.map(l => l.textContent.trim());

    // Buttons — are all action buttons <button> not <a>?
    const navLinks = [...document.querySelectorAll(".rows .row a")];
    const drillActionLinks = navLinks.filter(a => a.href && !a.href.includes("#"));
    const hasHrefDrillActions = drillActionLinks.length > 0;

    // Empty state
    const emptyEl = document.querySelector(".empty,[class*=empty]");
    const hasEmpty = !!emptyEl;
    const emptyText = emptyEl ? emptyEl.textContent.trim() : "N/A";

    // Page errors
    const errEl = document.querySelector(".err,[role=alert]");
    const errText = errEl ? errEl.textContent.trim() : "";

    const allBtnTexts = [...document.querySelectorAll("button")].map(b => b.textContent.trim()).slice(0, 40);

    return {
      cardTitles, cardCount: cards.length,
      rowCount, gripCount, gripTitles,
      warnCount, warnTexts,
      hasMermaid, mermaidAriaLabel,
      hasSuggestCard, suggestTitle, suggestRowCount, suggestBtns,
      listCardTitle, hasApplyBtn, applyBtnClass, applyBtnText,
      coverageText, editBtnCount,
      hasAddForm, addFormFieldCount,
      overflowCount, dataTheme,
      cardTitleColor, cardBg, pageBg,
      rowDescColor, rowMetaColor, mutedColor,
      labelTexts,
      hasHrefDrillActions,
      hasEmpty, emptyText,
      errText, allBtnTexts,
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

    // Row desc (rule match text)
    const rowDescEl = document.querySelector(".rows .row .row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowBg = rowDescEl ? getComputedStyle(rowDescEl.closest(".row") || document.body).backgroundColor : "N/A";

    // Row meta (applies-to text)
    const rowMetaEl = document.querySelector(".rows .row .row-meta");
    const rowMetaColor = rowMetaEl ? getComputedStyle(rowMetaEl).color : "N/A";

    // Warning span
    const warnEl = document.querySelector(".row-meta[class*=warn],[class*=text-warn]");
    const warnColor = warnEl ? getComputedStyle(warnEl).color : "N/A";

    // Muted text
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Field inputs
    const fieldEl = document.querySelector("input.field,select.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    // Body bg between cards (systemic bleed check)
    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Apply-existing btn
    const applyBtn = [...document.querySelectorAll("button")].find(b =>
      b.textContent.toLowerCase().includes("apply")
    );
    const applyBtnColor = applyBtn ? getComputedStyle(applyBtn).color : "N/A";
    const applyBtnBg = applyBtn ? getComputedStyle(applyBtn).backgroundColor : "N/A";

    // Suggest btn
    const suggestBtn = [...document.querySelectorAll("button")].find(b =>
      b.textContent.toLowerCase() === "add" || b.className.includes("primary")
    );
    const suggestBtnColor = suggestBtn ? getComputedStyle(suggestBtn).color : "N/A";
    const suggestBtnBg = suggestBtn ? getComputedStyle(suggestBtn).backgroundColor : "N/A";

    // Edit button
    const editBtn = [...document.querySelectorAll("button")].find(b =>
      b.textContent.toLowerCase() === "edit" || b.title?.toLowerCase().includes("edit")
    );
    const editBtnColor = editBtn ? getComputedStyle(editBtn).color : "N/A";
    const editBtnBg = editBtn ? getComputedStyle(editBtn).backgroundColor : "N/A";

    return {
      dataTheme, pageBg,
      cardTitleColor, cardBg,
      rowDescColor, rowBg, rowMetaColor,
      warnColor,
      mutedColor,
      fieldBg, fieldColor,
      mainBg,
      applyBtnColor, applyBtnBg,
      suggestBtnColor, suggestBtnBg,
      editBtnColor, editBtnBg,
    };
  });
}

// ---------------------------------------------------------------
// Exercise: inline-edit a rule row
// ---------------------------------------------------------------
async function exerciseInlineEdit(page) {
  const editBtn = page.locator('button[title*="Edit"],button[title*="edit"]').first();
  if (await editBtn.count() > 0) {
    await editBtn.click();
    await page.waitForTimeout(600);
    return true;
  }
  return false;
}

try {
  // ============================================================
  // DARK THEME SESSION
  // ============================================================
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await dark.setViewportSize({ width: 1280, height: 900 });
  await navToRules(dark);

  // Screenshot: full list view at 1280 dark
  await dark.screenshot({ path: shot("dark_1280") });
  await dark.screenshot({ path: shot("dark_1280_full"), fullPage: true });

  const domDark = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_18_rules_dom_dark.json"), JSON.stringify(domDark, null, 2));
  console.log("DOM (dark):", JSON.stringify(domDark, null, 2));

  // Exercise: inline edit a row
  const didEdit = await exerciseInlineEdit(dark);
  if (didEdit) {
    await dark.screenshot({ path: shot("dark_1280_inline_edit") });
    await dark.screenshot({ path: shot("dark_1280_inline_edit_full"), fullPage: true });
    // Cancel edit
    const cancelBtn = dark.locator('button[type="button"]').filter({ hasText: /cancel/i }).first();
    if (await cancelBtn.count() > 0) await cancelBtn.click();
    await dark.waitForTimeout(400);
  }

  // 1440px dark
  await dark.setViewportSize({ width: 1440, height: 900 });
  await navToRules(dark);
  await dark.screenshot({ path: shot("dark_1440") });
  await dark.screenshot({ path: shot("dark_1440_full"), fullPage: true });

  // 768px dark
  await dark.setViewportSize({ width: 768, height: 900 });
  await navToRules(dark);
  await dark.screenshot({ path: shot("dark_768") });
  await dark.screenshot({ path: shot("dark_768_full"), fullPage: true });

  await dark.close();

  // ============================================================
  // LIGHT THEME SESSION
  // ============================================================
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));
  await light.setViewportSize({ width: 1280, height: 900 });
  await navToRules(light);
  await applyLight(light);
  await navToRules(light);

  await light.screenshot({ path: shot("light_1280") });
  await light.screenshot({ path: shot("light_1280_full"), fullPage: true });

  const domLight = await auditDOM(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_18_rules_dom_light.json"), JSON.stringify(domLight, null, 2));
  console.log("DOM (light):", JSON.stringify(domLight, null, 2));

  const contrastLight = await auditLightContrast(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_18_rules_light_contrast.json"), JSON.stringify(contrastLight, null, 2));
  console.log("Light contrast:", JSON.stringify(contrastLight, null, 2));

  // Exercise inline edit in light
  const didEditLight = await exerciseInlineEdit(light);
  if (didEditLight) {
    await light.screenshot({ path: shot("light_1280_inline_edit") });
    await light.screenshot({ path: shot("light_1280_inline_edit_full"), fullPage: true });
    const cancelBtn = light.locator('button[type="button"]').filter({ hasText: /cancel/i }).first();
    if (await cancelBtn.count() > 0) await cancelBtn.click();
    await light.waitForTimeout(400);
  }

  // 1440px light
  await light.setViewportSize({ width: 1440, height: 900 });
  await navToRules(light);
  await light.screenshot({ path: shot("light_1440") });
  await light.screenshot({ path: shot("light_1440_full"), fullPage: true });

  // 768px light
  await light.setViewportSize({ width: 768, height: 900 });
  await navToRules(light);
  await light.screenshot({ path: shot("light_768") });
  await light.screenshot({ path: shot("light_768_full"), fullPage: true });

  await light.close();

} catch (e) {
  console.error("FATAL:", e);
  process.exit(1);
} finally {
  await browser.close();
}

if (errors.length > 0) {
  console.warn("Page errors:", errors);
}

console.log("G18 screenshots written to", SHOTS);
