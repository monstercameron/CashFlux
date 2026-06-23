// GLAMOR G19 — Workflows page visual + structural review for "The Automator" (Raj).
// Reviews the workflow list, builder (name/trigger/condition/actions), staged-action display,
// dry-run preview, mermaid flowchart, run history, light-mode contrast, and 768px behaviour.
// Screenshots at 1280 / 1440 / 768 × dark + light.
// Writes into e2e/screenshots/glamor_19_workflows_*.png and glamor_19_workflows_dom.json.
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

const shot = (name) => path.join(SHOTS, `glamor_19_workflows_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------
async function navToWorkflows(page) {
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
  const link = page.locator('nav a[title="Workflows"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /workflows/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/workflows", { waitUntil: "domcontentloaded" });
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
}

// ---------------------------------------------------------------
// DOM audit
// ---------------------------------------------------------------
async function auditDOM(page) {
  return page.evaluate(() => {
    // Cards
    const cards = [...document.querySelectorAll(".card, section.card")];
    const cardTitles = cards.map(c => {
      const t = c.querySelector("h2,h3,.card-title");
      return t ? t.textContent.trim() : "(no title)";
    });
    const cardHeadingLevels = cards.map(c => {
      const t = c.querySelector("h1,h2,h3,h4");
      return t ? t.tagName : "none";
    });

    // Buttons
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim());

    // Inputs / selects / fields
    const fields = [...document.querySelectorAll('input.field,select.field,textarea.field')];
    const fieldCount = fields.length;
    const fieldPlaceholders = fields.map(f => f.placeholder || f.getAttribute("aria-label") || "(none)");

    // Labels
    const labels = [...document.querySelectorAll("label")];
    const labelTexts = labels.map(l => l.textContent.trim());

    // Builder: detect name / trigger / condition inputs
    const nameInput = fields.find(f => (f.placeholder || "").toLowerCase().includes("name") || (f.getAttribute("aria-label") || "").toLowerCase().includes("name"));
    const conditionInput = fields.find(f => (f.placeholder || "").toLowerCase().includes("condition") || (f.getAttribute("aria-label") || "").toLowerCase().includes("condition"));
    const hasNameInput = !!nameInput;
    const hasConditionInput = !!conditionInput;
    const namePlaceholder = nameInput ? (nameInput.placeholder || "") : "N/A";
    const conditionPlaceholder = conditionInput ? (conditionInput.placeholder || "") : "N/A";

    // Action builder controls
    const hasActionKindSelect = fields.some(f => f.tagName === "SELECT");
    const addActionBtn = btns.find(b => b.textContent.trim().toLowerCase().includes("add action") || b.textContent.trim().toLowerCase().includes("add"));
    const hasSaveBtn = btns.some(b => b.textContent.trim().toLowerCase().includes("save") || b.textContent.trim().toLowerCase() === "save");
    const hasDryRunBtn = btns.some(b => b.textContent.trim().toLowerCase().includes("dry run") || b.textContent.trim().toLowerCase().includes("dry-run"));
    const hasRunNowBtn = btns.some(b => b.textContent.trim().toLowerCase().includes("run now"));
    const hasDeleteBtn = btns.some(b => b.textContent.trim() === "✕" || b.textContent.trim() === "×" || b.getAttribute("aria-label")?.toLowerCase().includes("delete"));
    const hasEditBtn = btns.some(b => b.textContent.trim().toLowerCase() === "edit" || b.getAttribute("aria-label")?.toLowerCase().includes("edit"));

    // Workflow list rows
    const allRows = [...document.querySelectorAll(".row,.row-edit")];
    const workflowRows = allRows.filter(r => {
      const btnsInRow = r.querySelectorAll("button");
      return btnsInRow.length >= 2;
    });
    const workflowRowCount = workflowRows.length;
    const workflowNames = workflowRows.map(r => r.querySelector(".row-desc")?.textContent?.trim() || "");

    // Staged actions (rows in builder area with remove button)
    const stagedRows = allRows.filter(r => {
      const remove = r.querySelector('button[aria-label="Remove action"]') || r.querySelector('.btn-del');
      return remove && !r.closest('.row-edit > div > div');
    });
    const stagedCount = stagedRows.length;

    // Mermaid flowchart
    const mermaidEls = [...document.querySelectorAll('pre.mermaid,div[class*="mermaid"],svg')];
    const hasMermaid = mermaidEls.length > 0;

    // Empty state
    const emptyEl = document.querySelector(".empty");
    const emptyText = emptyEl ? emptyEl.textContent.trim() : "N/A";
    const hasEmptyState = !!emptyEl;

    // History card
    const historyCard = cards.find(c => {
      const t = c.querySelector("h2,h3,.card-title");
      return t && t.textContent.toLowerCase().includes("history");
    });
    const hasHistoryCard = !!historyCard;
    const historyRowCount = historyCard ? historyCard.querySelectorAll(".row").length : 0;

    // Overflow
    const overflowCount = [...document.querySelectorAll(".card,section,.row")].filter(c => c.scrollWidth > c.clientWidth + 4).length;
    const pageHeight = document.body.scrollHeight;
    const viewportH = window.innerHeight;

    // Theming
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const cardTitleEl = document.querySelector("h2.card-title,h3.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card,section.card") || document.body).backgroundColor : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Row text colors
    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowMetaEl = document.querySelector(".row-meta");
    const rowMetaColor = rowMetaEl ? getComputedStyle(rowMetaEl).color : "N/A";

    // Field styles
    const fieldEl = document.querySelector("input.field,textarea.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    // Main content bg (for bleed check)
    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Page error
    const errEl = document.querySelector(".err,[role=alert]");
    const errText = errEl ? errEl.textContent.trim() : "";

    return {
      cardCount: cards.length, cardTitles, cardHeadingLevels, cardOrder: cardTitles,
      fieldCount, fieldPlaceholders,
      labelTexts, labelCount: labels.length,
      hasNameInput, namePlaceholder,
      hasConditionInput, conditionPlaceholder,
      hasActionKindSelect,
      hasSaveBtn, hasDryRunBtn, hasRunNowBtn, hasDeleteBtn, hasEditBtn,
      workflowRowCount, workflowNames,
      stagedCount,
      hasMermaid,
      hasEmptyState, emptyText,
      hasHistoryCard, historyRowCount,
      overflowCount, pageHeight, viewportH,
      dataTheme, cardTitleColor, cardBg, pageBg, mutedColor,
      rowDescColor, rowMetaColor,
      fieldBg, fieldColor, mainBg,
      errText,
      btnTexts: btnTexts.slice(0, 40),
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

    const cardTitleEl = document.querySelector("h2.card-title,h3.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card,section.card") || document.body).backgroundColor : "N/A";

    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowDescBg = rowDescEl ? getComputedStyle(rowDescEl.closest(".row,.row-edit") || document.body).backgroundColor : "N/A";

    const rowMetaEl = document.querySelector(".row-meta");
    const rowMetaColor = rowMetaEl ? getComputedStyle(rowMetaEl).color : "N/A";

    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    const fieldEl = document.querySelector("input.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Dry-run result text
    const dryResultEl = document.querySelector(".row-meta");
    const dryResultColor = dryResultEl ? getComputedStyle(dryResultEl).color : "N/A";

    // btn colors
    const btnPrimaryEl = document.querySelector(".btn-primary");
    const btnPrimaryColor = btnPrimaryEl ? getComputedStyle(btnPrimaryEl).color : "N/A";
    const btnPrimaryBg = btnPrimaryEl ? getComputedStyle(btnPrimaryEl).backgroundColor : "N/A";

    const btnEl = document.querySelector(".btn:not(.btn-primary):not(.btn-del)");
    const btnColor = btnEl ? getComputedStyle(btnEl).color : "N/A";
    const btnBg = btnEl ? getComputedStyle(btnEl).backgroundColor : "N/A";

    return {
      dataTheme, pageBg,
      cardTitleColor, cardBg,
      rowDescColor, rowDescBg,
      rowMetaColor,
      mutedColor,
      fieldBg, fieldColor,
      mainBg,
      dryResultColor,
      btnPrimaryColor, btnPrimaryBg,
      btnColor, btnBg,
    };
  });
}

// ---------------------------------------------------------------
// Build a workflow and get dry-run result
// ---------------------------------------------------------------
async function buildWorkflow(page, label) {
  // Fill name
  const nameInput = page.locator('input.field').first();
  if (await nameInput.count() > 0) {
    await nameInput.fill(`Monthly check ${label}`);
    await page.waitForTimeout(200);
  }

  // Set trigger to "When a transaction is added" (second option)
  const triggerSelect = page.locator('select.field').first();
  if (await triggerSelect.count() > 0) {
    await triggerSelect.selectOption({ index: 1 });
    await page.waitForTimeout(200);
  }

  // Fill condition
  const conditionInput = page.locator('input.field').nth(1);
  if (await conditionInput.count() > 0) {
    await conditionInput.fill("expense > 100");
    await page.waitForTimeout(200);
  }

  // The action kind select is in the second form-grid
  // Default is "Create a task" — just fill the action text
  const actionTextInput = page.locator('input.field').nth(2);
  if (await actionTextInput.count() > 0) {
    await actionTextInput.fill("Review large expense");
    await page.waitForTimeout(200);
  }

  // Click "Add action"
  const addBtn = page.locator('button').filter({ hasText: /add action/i }).first();
  if (await addBtn.count() > 0) {
    await addBtn.click();
    await page.waitForTimeout(400);
  }
}

try {
  // ============================================================
  // DARK THEME SESSION
  // ============================================================
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await dark.setViewportSize({ width: 1280, height: 900 });
  await navToWorkflows(dark);

  // Screenshot: empty/list state at 1280 dark
  await dark.screenshot({ path: shot("dark_1280_list") });
  await dark.screenshot({ path: shot("dark_1280_list_full"), fullPage: true });

  // DOM audit (initial state)
  const domAudit = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_19_workflows_dom.json"), JSON.stringify(domAudit, null, 2));
  console.log("[dark DOM audit]", JSON.stringify(domAudit, null, 2));

  // Build a workflow
  await buildWorkflow(dark, "dark");
  await dark.screenshot({ path: shot("dark_1280_builder") });
  await dark.screenshot({ path: shot("dark_1280_builder_full"), fullPage: true });

  // DOM audit after building (staged action visible)
  const domAuditBuilt = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_19_workflows_dom_built.json"), JSON.stringify(domAuditBuilt, null, 2));
  console.log("[dark DOM audit (built)]", JSON.stringify(domAuditBuilt, null, 2));

  // Save the workflow
  const saveBtn = dark.locator('button.btn-primary').filter({ hasText: /save/i }).first();
  if (await saveBtn.count() > 0) {
    await saveBtn.click();
    await dark.waitForTimeout(1000);
    await dark.screenshot({ path: shot("dark_1280_saved") });
  }

  // DOM audit after save (workflow list should have an entry)
  const domAuditSaved = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_19_workflows_dom_saved.json"), JSON.stringify(domAuditSaved, null, 2));

  // Dry-run: click the first "Dry run" button in the list
  const dryRunBtn = dark.locator('button').filter({ hasText: /dry.?run/i }).first();
  if (await dryRunBtn.count() > 0) {
    await dryRunBtn.click();
    await dark.waitForTimeout(800);
    await dark.screenshot({ path: shot("dark_1280_dryrun") });
    await dark.screenshot({ path: shot("dark_1280_dryrun_full"), fullPage: true });
  } else {
    console.log("[warn] No dry-run button found after save");
  }

  // 1440 dark
  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("dark_1440") });

  // 768 dark
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

  await navToWorkflows(light);
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(400);

  // Screenshot: list state light 1280
  await light.screenshot({ path: shot("light_1280_list") });
  await light.screenshot({ path: shot("light_1280_list_full"), fullPage: true });

  // Light contrast audit (before building, so the builder is visible)
  const lightContrast = await auditLightContrast(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_19_workflows_light_contrast.json"), JSON.stringify(lightContrast, null, 2));
  console.log("[light contrast]", JSON.stringify(lightContrast, null, 2));

  // Build a workflow in light
  await buildWorkflow(light, "light");
  await light.screenshot({ path: shot("light_1280_builder") });

  // Save
  const lightSaveBtn = light.locator('button.btn-primary').filter({ hasText: /save/i }).first();
  if (await lightSaveBtn.count() > 0) {
    await lightSaveBtn.click();
    await light.waitForTimeout(1000);
    await light.screenshot({ path: shot("light_1280_saved") });
  }

  // Dry-run in light
  const lightDryBtn = light.locator('button').filter({ hasText: /dry.?run/i }).first();
  if (await lightDryBtn.count() > 0) {
    await lightDryBtn.click();
    await light.waitForTimeout(800);
    await light.screenshot({ path: shot("light_1280_dryrun") });
    await light.screenshot({ path: shot("light_1280_dryrun_full"), fullPage: true });
  }

  // Light contrast audit post-save (list now has rows)
  const lightContrastPost = await auditLightContrast(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_19_workflows_light_contrast_post.json"), JSON.stringify(lightContrastPost, null, 2));
  console.log("[light contrast post-save]", JSON.stringify(lightContrastPost, null, 2));

  // 1440 light
  await light.setViewportSize({ width: 1440, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("light_1440") });

  // 768 light
  await light.setViewportSize({ width: 768, height: 1024 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("light_768") });
  await light.screenshot({ path: shot("light_768_full"), fullPage: true });

  // ============================================================
  // Summarise
  // ============================================================
  if (errors.length > 0) {
    console.error("\n[PAGE ERRORS]", errors);
    process.exit(1);
  }

  const shots = [
    "dark_1280_list", "dark_1280_list_full",
    "dark_1280_builder", "dark_1280_builder_full",
    "dark_1280_saved", "dark_1280_dryrun", "dark_1280_dryrun_full",
    "dark_1440", "dark_768", "dark_768_full",
    "light_1280_list", "light_1280_list_full",
    "light_1280_builder",
    "light_1280_saved", "light_1280_dryrun", "light_1280_dryrun_full",
    "light_1440", "light_768", "light_768_full",
  ];
  console.log("\n[screenshots produced]");
  for (const s of shots) {
    const p = shot(s);
    console.log(" ", fs.existsSync(p) ? "✓" : "✗", path.basename(p));
  }

  console.log("\n[ok] G19 Workflows review complete. Exit 0.");
} catch (err) {
  console.error("[FATAL]", err);
  process.exit(1);
} finally {
  await browser.close();
}
