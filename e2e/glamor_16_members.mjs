// GLAMOR G16 — Members page visual + structural review for "The Household Roster" (Renu).
// Reviews the member roster (cards, avatars, colors, actions), add form, color picker (C8),
// reassign-on-delete flow (C62), net-worth per owner, light-mode contrast, and 768px behaviour.
// Screenshots at 1280 / 1440 / 768 × dark + light.
// Writes into e2e/screenshots/glamor_16_members_*.png and glamor_16_members_dom.json.
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

const shot = (name) => path.join(SHOTS, `glamor_16_members_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------
async function navToMembers(page) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(600);
  // Reset "View as member" to Everyone
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
  const link = page.locator('nav a[title="Members"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /members/i }).first();
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

    // Member roster rows
    const rows = [...document.querySelectorAll(".row")];
    const rowCount = rows.length;

    // Avatars (.member-avatar)
    const avatarEls = [...document.querySelectorAll(".member-avatar")];
    const avatarCount = avatarEls.length;
    const avatarInitials = avatarEls.map(a => a.textContent.trim());
    const avatarStyles = avatarEls.map(a => a.getAttribute("style") || "");

    // Member names in rows
    const rowDescs = [...document.querySelectorAll(".row-desc")];
    const memberNames = rowDescs.map(d => d.textContent.trim()).filter(n => n.length > 0 && n.length < 80);

    // Badges
    const badges = [...document.querySelectorAll(".badge")];
    const badgeTexts = badges.map(b => b.textContent.trim());

    // Action buttons per row (non-delete)
    const allBtns = [...document.querySelectorAll("button")];
    const btnTexts = allBtns.map(b => b.textContent.trim());
    const delBtns = [...document.querySelectorAll(".btn-del")];
    const delBtnCount = delBtns.length;

    // Check button types (must be <button> not <a>)
    const actionLinks = [...document.querySelectorAll(".row a[href]")];
    const actionLinkCount = actionLinks.length;

    // Color picker inputs
    const colorInputs = [...document.querySelectorAll('input[type="color"]')];
    const colorInputCount = colorInputs.length;
    // Look for color input in add form
    const addFormColorInput = document.querySelector('[data-testid="member-add-form"] input[type="color"]');
    const hasAddFormColorInput = !!addFormColorInput;
    const addFormColorValue = addFormColorInput ? addFormColorInput.value : "N/A";

    // Add form
    const addForm = document.querySelector('[data-testid="member-add-form"]');
    const hasAddForm = !!addForm;
    const addFormInputs = addForm ? [...addForm.querySelectorAll('input')] : [];
    const addFormInputTypes = addFormInputs.map(i => i.type);
    const addBtn = addForm ? [...addForm.querySelectorAll('button')].find(b => b.type === "submit") : null;
    const addBtnText = addBtn ? addBtn.textContent.trim() : "N/A";

    // Labels
    const allLabels = [...document.querySelectorAll("label")];
    const labelTexts = allLabels.map(l => l.textContent.trim()).filter(t => t);

    // Net worth card
    const netWorthCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && (t.textContent.toLowerCase().includes("net worth") || t.textContent.toLowerCase().includes("owner"));
    });
    const hasNetWorthCard = !!netWorthCard;
    const netWorthRows = netWorthCard ? [...netWorthCard.querySelectorAll(".row")] : [];
    const netWorthRowCount = netWorthRows.length;
    const netWorthAmounts = netWorthRows.map(r => {
      const amt = r.querySelector(".amount,.fig,.pos,.neg");
      return amt ? amt.textContent.trim() : "";
    });

    // Reassign panel
    const reassignCard = cards.find(c => {
      const t = c.querySelector("h2,.card-title");
      return t && (t.textContent.toLowerCase().includes("reassign") || t.textContent.toLowerCase().includes("before") || t.textContent.toLowerCase().includes("move"));
    });
    const hasReassignPanel = !!reassignCard;
    const reassignSelects = [...document.querySelectorAll("select.field")];
    const reassignSelectCount = reassignSelects.length;

    // Layout / overflow
    const overflowCount = cards.filter(c => c.scrollWidth > c.clientWidth + 4).length;
    const pageHeight = document.body.scrollHeight;
    const viewportH = window.innerHeight;

    // Theming
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";

    // Colors of key elements
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || document.body).backgroundColor : "N/A";
    const pageBg = getComputedStyle(document.body).backgroundColor;

    // Row text colors
    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowMetaEl = document.querySelector(".row-meta");
    const rowMetaColor = rowMetaEl ? getComputedStyle(rowMetaEl).color : "N/A";

    // Amount colors in net-worth section
    const amtEl = netWorthCard ? netWorthCard.querySelector(".amount,.fig,.pos,.neg") : document.querySelector(".amount,.fig");
    const amtColor = amtEl ? getComputedStyle(amtEl).color : "N/A";
    const amtBg = amtEl ? getComputedStyle(amtEl.closest(".row") || document.body).backgroundColor : "N/A";

    // Muted text
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Main container bg (to detect body-bg bleed)
    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Field inputs
    const fieldEl = document.querySelector("input.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    // Check for any anchor-based drill actions (probe C62/G16 rule)
    const hrefActions = [...document.querySelectorAll('.row a[href],.row-actions a[href]')];
    const hrefActionCount = hrefActions.length;

    const errEl = document.querySelector(".err,[role=alert]");
    const errText = errEl ? errEl.textContent.trim() : "";

    return {
      cardTitles, cardCount: cards.length,
      rowCount, memberNames,
      avatarCount, avatarInitials, avatarStyles,
      badgeTexts,
      btnTexts: btnTexts.slice(0, 40),
      delBtnCount,
      actionLinkCount, hrefActionCount,
      colorInputCount, hasAddFormColorInput, addFormColorValue,
      hasAddForm, addFormInputTypes, addBtnText,
      labelTexts,
      hasNetWorthCard, netWorthRowCount, netWorthAmounts,
      hasReassignPanel, reassignSelectCount,
      overflowCount, pageHeight, viewportH,
      dataTheme,
      cardTitleColor, cardBg, pageBg,
      rowDescColor, rowMetaColor,
      amtColor, amtBg,
      mutedColor,
      mainBg,
      fieldBg, fieldColor,
      errText,
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

    // Member name row
    const rowDescEl = document.querySelector(".rows .row .row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const rowBg = rowDescEl ? getComputedStyle(rowDescEl.closest(".row") || document.body).backgroundColor : "N/A";

    // Row meta (role label)
    const rowMetaEl = document.querySelector(".rows .row .row-meta");
    const rowMetaColor = rowMetaEl ? getComputedStyle(rowMetaEl).color : "N/A";

    // Amount in net worth rows
    const amtEl = document.querySelector(".rows .row .amount,.rows .row .fig,.rows .row .pos,.rows .row .neg");
    const amtColor = amtEl ? getComputedStyle(amtEl).color : "N/A";
    const amtBg = amtEl ? getComputedStyle(amtEl.closest(".row") || document.body).backgroundColor : "N/A";

    // Color picker (C8)
    const colorInputEl = document.querySelector('input[type="color"]');
    const colorInputBg = colorInputEl ? getComputedStyle(colorInputEl).backgroundColor : "N/A";
    const colorInputBorder = colorInputEl ? getComputedStyle(colorInputEl).borderColor : "N/A";
    const colorInputHeight = colorInputEl ? getComputedStyle(colorInputEl).height : "N/A";
    const colorInputWidth = colorInputEl ? getComputedStyle(colorInputEl).width : "N/A";

    // Muted text
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";

    // Field
    const fieldEl = document.querySelector("input.field");
    const fieldBg = fieldEl ? getComputedStyle(fieldEl).backgroundColor : "N/A";
    const fieldColor = fieldEl ? getComputedStyle(fieldEl).color : "N/A";

    // Body bg bleed between cards
    const mainEl = document.querySelector("main,.main-content,[class*=content]");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "N/A";

    // Badge
    const badgeEl = document.querySelector(".badge");
    const badgeColor = badgeEl ? getComputedStyle(badgeEl).color : "N/A";
    const badgeBg = badgeEl ? getComputedStyle(badgeEl).backgroundColor : "N/A";

    return {
      dataTheme, pageBg,
      cardTitleColor, cardBg,
      rowDescColor, rowBg,
      rowMetaColor,
      amtColor, amtBg,
      colorInputBg, colorInputBorder, colorInputHeight, colorInputWidth,
      mutedColor,
      fieldBg, fieldColor,
      mainBg,
      badgeColor, badgeBg,
    };
  });
}

// ---------------------------------------------------------------
// Exercise: open add form inline, then trigger inline edit
// ---------------------------------------------------------------
async function exerciseAddForm(page) {
  // Look for a nav "+" button or "Add member" button
  const addBtn = page.locator('button').filter({ hasText: /add.*member|new.*member|\+/i }).first();
  if (await addBtn.count() > 0) {
    await addBtn.click();
    await page.waitForTimeout(500);
    return true;
  }
  // If the add form is already shown (e.g. embedded in the card), just screenshot
  const addForm = page.locator('[data-testid="member-add-form"]');
  return (await addForm.count()) > 0;
}

async function exerciseInlineEdit(page) {
  // Click the first "Edit" button in a member row
  const editBtn = page.locator('button').filter({ hasText: /^edit$/i }).first();
  if (await editBtn.count() > 0) {
    await editBtn.click();
    await page.waitForTimeout(500);
    return true;
  }
  // Fallback: button with pencil icon
  const pencilBtn = page.locator('button[title*="Edit"],button[title*="edit"]').first();
  if (await pencilBtn.count() > 0) {
    await pencilBtn.click();
    await page.waitForTimeout(500);
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
  await navToMembers(dark);

  // Screenshot: roster landing at 1280 dark
  await dark.screenshot({ path: shot("dark_1280_roster") });
  await dark.screenshot({ path: shot("dark_1280_roster_full"), fullPage: true });

  // DOM audit (roster state)
  const domAudit = await auditDOM(dark);
  fs.writeFileSync(path.join(SHOTS, "glamor_16_members_dom.json"), JSON.stringify(domAudit, null, 2));
  console.log("[dark DOM audit]", JSON.stringify(domAudit, null, 2));

  // Exercise inline edit (to see color picker rendered)
  const didEdit = await exerciseInlineEdit(dark);
  if (didEdit) {
    await dark.screenshot({ path: shot("dark_1280_edit") });
  }

  // Screenshot at 1440
  await dark.setViewportSize({ width: 1440, height: 900 });
  // Re-navigate to reset to roster state
  await navToMembers(dark);
  await dark.screenshot({ path: shot("dark_1440") });
  await dark.screenshot({ path: shot("dark_1440_full"), fullPage: true });

  // Screenshot at 768
  await dark.setViewportSize({ width: 768, height: 1024 });
  await navToMembers(dark);
  await dark.screenshot({ path: shot("dark_768") });
  await dark.screenshot({ path: shot("dark_768_full"), fullPage: true });

  // ============================================================
  // LIGHT THEME SESSION
  // ============================================================
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));

  // Light theme recipe (canonical from instructions)
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

  // Navigate to Members in light mode
  const lightLink = light.locator('nav a[title="Members"]').first();
  if (await lightLink.count() > 0) {
    await lightLink.click();
  } else {
    const fb = light.locator('nav a').filter({ hasText: /members/i }).first();
    if (await fb.count() > 0) await fb.click();
  }
  await light.waitForSelector(".card", { timeout: 30000 });
  await light.waitForTimeout(1000);

  // Screenshots at 1280 light
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_1280_roster") });
  await light.screenshot({ path: shot("light_1280_roster_full"), fullPage: true });

  // DOM audit in light (for label/contrast check)
  const domAuditLight = await auditDOM(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_16_members_dom_light.json"), JSON.stringify(domAuditLight, null, 2));

  // Exercise inline edit in light (to see color picker in light theme — C8)
  const didEditLight = await exerciseInlineEdit(light);
  if (didEditLight) {
    await light.screenshot({ path: shot("light_1280_edit") });
  }

  // Light contrast audit
  const lightContrast = await auditLightContrast(light);
  fs.writeFileSync(path.join(SHOTS, "glamor_16_members_light_contrast.json"), JSON.stringify(lightContrast, null, 2));
  console.log("[light contrast]", JSON.stringify(lightContrast, null, 2));

  // At 1440 light
  await light.setViewportSize({ width: 1440, height: 900 });
  // Reset to roster (cancel any edit)
  await navToMembers(light);
  await light.waitForFunction(() => document.documentElement.getAttribute("data-theme") === "light", { timeout: 10000 });
  await light.screenshot({ path: shot("light_1440") });
  await light.screenshot({ path: shot("light_1440_full"), fullPage: true });

  // At 768 light
  await light.setViewportSize({ width: 768, height: 1024 });
  await navToMembers(light);
  await light.waitForFunction(() => document.documentElement.getAttribute("data-theme") === "light", { timeout: 10000 });
  await light.screenshot({ path: shot("light_768") });
  await light.screenshot({ path: shot("light_768_full"), fullPage: true });

  // ============================================================
  // Summarise
  // ============================================================
  console.log("\n--- GLAMOR G16 MEMBERS DOM AUDIT (dark) ---");
  console.log(JSON.stringify(domAudit, null, 2));

  if (errors.length > 0) {
    console.error("\n[PAGE ERRORS]", errors);
    process.exit(1);
  }

  const shots = [
    "dark_1280_roster", "dark_1280_roster_full", "dark_1280_edit",
    "dark_1440", "dark_1440_full", "dark_768", "dark_768_full",
    "light_1280_roster", "light_1280_roster_full", "light_1280_edit",
    "light_1440", "light_1440_full", "light_768", "light_768_full",
  ];
  console.log("\n[screenshots produced]");
  for (const s of shots) {
    const p = shot(s);
    console.log(" ", fs.existsSync(p) ? "✓" : "✗", path.basename(p));
  }

  console.log("\n[ok] G16 Members review complete. Exit 0.");
} catch (err) {
  console.error("[FATAL]", err);
  process.exit(1);
} finally {
  await browser.close();
}
