// GLAMOR G17 — Categories page visual + structural review for "The Category Nerd" (Tomás).
// Reviews the tree structure (parent + sub-categories), kind grouping (Expense/Income),
// add/edit inline forms, indentation style, usage count badges, light-mode contrast,
// and responsive behaviour at 768px.
// Screenshots at 1280 / 1440 / 768 × dark + light.
// Writes into e2e/screenshots/glamor_17_categories_*.png and glamor_17_categories_dom.json.
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

const shot = (name) => path.join(SHOTS, `glamor_17_categories_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------
async function navToCategories(page) {
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
  // Hard-reload to ensure fresh state
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(600);
  // Navigate via nav link
  const link = page.locator('nav a[title="Categories"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /categor/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
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

    // Category rows
    const rows = [...document.querySelectorAll(".row")];
    const rowCount = rows.length;

    // Row labels / names
    const rowDescs = [...document.querySelectorAll(".row-desc")];
    const rowLabels = rowDescs.map(el => el.textContent.trim());

    // Em-dash prefixes (C63 — should be fixed but checking)
    const emDashRows = rowLabels.filter(t => t.startsWith("—") || t.startsWith("–"));

    // Indented/nested rows — look for paddingLeft style or class
    const indentedRows = [...document.querySelectorAll(".row-desc")].filter(
      el => parseFloat(el.style.paddingLeft || "0") > 0 || el.className.includes("indent") || el.className.includes("nested")
    );
    const indentedCount = indentedRows.length;

    // Kind grouping — expense / income sections
    const kindHeaders = [...document.querySelectorAll("h2,h3,.section-header,.kind-header")]
      .map(el => el.textContent.trim())
      .filter(t => /expense|income/i.test(t));

    // Usage count badges
    const usageBadges = [...document.querySelectorAll(".cat-usage,[data-testid*='usage'],.usage-badge,.badge")];
    const usageBadgeCount = usageBadges.length;
    const usageBadgeTexts = usageBadges.map(el => el.textContent.trim()).slice(0, 10);

    // Add form elements
    const addButtons = [...document.querySelectorAll("button")].filter(b =>
      /add|new|\+/i.test(b.textContent.trim())
    );
    const addBtnTexts = addButtons.map(b => b.textContent.trim());

    // Edit/Delete buttons — check if they are <button> elements (not <a href>)
    const allBtns = [...document.querySelectorAll("button")];
    const actionBtnTexts = allBtns.map(b => b.textContent.trim());
    const editBtns = allBtns.filter(b => /edit/i.test(b.textContent.trim()));
    const deleteBtns = allBtns.filter(b => /delete|remove/i.test(b.textContent.trim()));
    const editBtnIsBtn = editBtns.length > 0; // all already button elements
    const editLinksAsHref = [...document.querySelectorAll("a")].filter(a =>
      /edit/i.test(a.textContent.trim()) && a.closest(".row")
    ).length;

    // Labels vs placeholder-only inputs in any open forms
    const allInputs = [...document.querySelectorAll("input,select,textarea")];
    const inputLabels = allInputs.map(el => {
      const id = el.id;
      const label = id ? document.querySelector(`label[for="${id}"]`) : null;
      const aria = el.getAttribute("aria-label") || el.getAttribute("aria-labelledby") || "";
      const placeholder = el.getAttribute("placeholder") || "";
      return { id, hasLabel: !!label, aria, placeholder, tag: el.tagName };
    });

    // Color swatches
    const colorSwatches = document.querySelectorAll(".color-swatch,.swatch,[style*='background-color']");
    const colorSwatchCount = colorSwatches.length;

    // Overflow
    const overflowCount = cards.filter(c => c.scrollWidth > c.clientWidth + 4).length;
    const pageHeight = document.body.scrollHeight;
    const viewportH = window.innerHeight;

    // Theming
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl
      ? getComputedStyle(cardTitleEl.closest(".card") || document.body).backgroundColor
      : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;

    // Row colors
    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowBg = rowDescEl
      ? getComputedStyle(rowDescEl.closest(".row") || document.body).backgroundColor
      : "N/A";

    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Page errors indicator
    const errEl = document.querySelector(".err,[role=alert]");
    const errText = errEl ? errEl.textContent.trim() : "";

    // Check empty state
    const emptyState = document.querySelector(".empty-state,.no-data,.empty");
    const hasEmptyState = !!emptyState;
    const emptyStateText = emptyState ? emptyState.textContent.trim() : "";

    // Expense vs income section breakdown
    const expenseSection = [...cards].find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && /expense/i.test(t.textContent);
    });
    const incomeSection = [...cards].find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && /income/i.test(t.textContent);
    });
    const hasExpenseSection = !!expenseSection;
    const hasIncomeSection = !!incomeSection;

    const expenseRows = expenseSection ? expenseSection.querySelectorAll(".row").length : 0;
    const incomeRows = incomeSection ? incomeSection.querySelectorAll(".row").length : 0;

    return {
      cardTitles, cardCount: cards.length,
      rowCount, rowLabels: rowLabels.slice(0, 20),
      emDashRows, indentedCount,
      kindHeaders, hasExpenseSection, hasIncomeSection,
      expenseRows, incomeRows,
      usageBadgeCount, usageBadgeTexts,
      addBtnTexts: addBtnTexts.slice(0, 10),
      editBtnIsBtn, editLinksAsHref,
      actionBtnTexts: actionBtnTexts.slice(0, 30),
      inputLabels,
      colorSwatchCount,
      overflowCount, pageHeight, viewportH,
      dataTheme, cardTitleColor, cardBg, pageBg,
      rowDescColor, rowBg, mutedColor,
      errText, hasEmptyState, emptyStateText,
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
    const cardBg = cardTitleEl
      ? getComputedStyle(cardTitleEl.closest(".card") || document.body).backgroundColor
      : "N/A";

    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowBg = rowDescEl
      ? getComputedStyle(rowDescEl.closest(".row") || document.body).backgroundColor
      : "N/A";

    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    const badgeEl = document.querySelector(".cat-usage,.badge,.usage-badge");
    const badgeColor = badgeEl ? getComputedStyle(badgeEl).color : "N/A";
    const badgeBg = badgeEl ? getComputedStyle(badgeEl).backgroundColor : "N/A";

    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Kind section header colors
    const kindHeaderEl = [...document.querySelectorAll("h2,h3,.section-header,.kind-header")]
      .find(el => /expense|income/i.test(el.textContent));
    const kindHeaderColor = kindHeaderEl ? getComputedStyle(kindHeaderEl).color : "N/A";
    const kindHeaderBg = kindHeaderEl
      ? getComputedStyle(kindHeaderEl.closest(".card") || document.body).backgroundColor
      : "N/A";

    // Indented row (sub-category) colors
    const indentedEl = [...document.querySelectorAll(".row-desc")]
      .find(el => parseFloat(el.style.paddingLeft || "0") > 0);
    const indentedColor = indentedEl ? getComputedStyle(indentedEl).color : "N/A";
    const indentedBg = indentedEl
      ? getComputedStyle(indentedEl.closest(".row") || document.body).backgroundColor
      : "N/A";

    return {
      dataTheme, pageBg,
      cardTitleColor, cardBg,
      rowDescColor, rowBg,
      mutedColor,
      badgeColor, badgeBg,
      mainBg,
      kindHeaderColor, kindHeaderBg,
      indentedColor, indentedBg,
    };
  });
}

// ---------------------------------------------------------------
// Exercise: open the Add form and screenshot it
// ---------------------------------------------------------------
async function openAddForm(page) {
  // Look for an add button scoped inside a .card (not the mobile quick-add tab)
  const cardAddBtn = page.locator(".card button").filter({ hasText: /add category|add new|new category/i }).first();
  if (await cardAddBtn.count() > 0 && await cardAddBtn.isVisible()) {
    await cardAddBtn.click();
    await page.waitForTimeout(600);
    return true;
  }
  // Try any visible btn-primary in a .card header area
  const primaryBtn = page.locator(".card .card-header button.btn-primary, .card h2 ~ button").first();
  if (await primaryBtn.count() > 0 && await primaryBtn.isVisible()) {
    await primaryBtn.click();
    await page.waitForTimeout(600);
    return true;
  }
  console.log("[info] openAddForm: could not find a visible add button in .card context, skipping");
  return false;
}

try {
  // ============================================================
  // DARK THEME SESSION
  // ============================================================
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await dark.setViewportSize({ width: 1280, height: 900 });
  await navToCategories(dark);

  // Screenshot: tree view at 1280 dark
  await dark.screenshot({ path: shot("dark_1280") });
  await dark.screenshot({ path: shot("dark_1280_full"), fullPage: true });

  // DOM audit
  const domAudit = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_17_categories_dom.json"), JSON.stringify(domAudit, null, 2));

  // Open Add form
  const addOpened = await openAddForm(dark);
  if (addOpened) {
    await dark.screenshot({ path: shot("dark_1280_add_form") });
    // Close by pressing Escape or clicking cancel
    await dark.keyboard.press("Escape");
    await dark.waitForTimeout(400);
  }

  // Screenshot at 1440 dark
  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_1440") });

  // Screenshot at 768 dark
  await dark.setViewportSize({ width: 768, height: 1024 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_768") });
  await dark.screenshot({ path: shot("dark_768_full"), fullPage: true });

  // ============================================================
  // LIGHT THEME SESSION
  // ============================================================
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));

  // Light theme recipe (canonical)
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
  await light.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "light",
    { timeout: 30000 }
  );
  await light.waitForTimeout(600);
  console.log("[ok] theme 'light' confirmed on <html>");

  // Navigate to Categories in light mode
  await light.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try {
        const p = JSON.parse(raw);
        delete p.viewAsMember;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      } catch (_) {}
    }
  });
  const lightLink = light.locator('nav a[title="Categories"]').first();
  if (await lightLink.count() > 0) {
    await lightLink.click();
  } else {
    const fb = light.locator('nav a').filter({ hasText: /categor/i }).first();
    if (await fb.count() > 0) await fb.click();
    else await light.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  }
  await light.waitForSelector(".card", { timeout: 30000 });
  await light.waitForTimeout(1200);

  // Screenshots at 1280 light
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_1280") });
  await light.screenshot({ path: shot("light_1280_full"), fullPage: true });

  // Open Add form in light
  const lightAddOpened = await openAddForm(light);
  if (lightAddOpened) {
    await light.screenshot({ path: shot("light_1280_add_form") });
    await light.keyboard.press("Escape");
    await light.waitForTimeout(400);
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
  fs.writeFileSync(
    path.join(SHOTS, "glamor_17_categories_light_contrast.json"),
    JSON.stringify(lightContrast, null, 2)
  );
  console.log("[light contrast]", JSON.stringify(lightContrast, null, 2));

  // ============================================================
  // Summarise
  // ============================================================
  console.log("\n--- GLAMOR G17 CATEGORIES DOM AUDIT (dark) ---");
  console.log(JSON.stringify(domAudit, null, 2));

  if (errors.length > 0) {
    console.error("\n[PAGE ERRORS]", errors);
    process.exit(1);
  }

  const shots = [
    "dark_1280", "dark_1280_full",
    "dark_1280_add_form",
    "dark_1440", "dark_768", "dark_768_full",
    "light_1280", "light_1280_full",
    "light_1280_add_form",
    "light_1440", "light_768", "light_768_full",
  ];
  console.log("\n[screenshots produced]");
  for (const s of shots) {
    const p = shot(s);
    console.log(" ", fs.existsSync(p) ? "✓" : "✗", path.basename(p));
  }

  console.log("\n[ok] G17 Categories review complete. Exit 0.");
} catch (err) {
  console.error("[FATAL]", err);
  process.exit(1);
} finally {
  await browser.close();
}
