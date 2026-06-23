// GLAMOR G21 — Settings panel visual + structural review for "Make It Mine" (Renée).
// Reviews the global settings fly-in FlipPanel: sections (household, AI, appearance,
// preferences, data), controls (theme/accent/density/week-start/date-format), import/export
// area, AI key area, module toggles, light-mode contrast and rendering, 768px behaviour.
// Screenshots at 1280 / 1440 / 768 × dark + light.
// Writes into e2e/screenshots/glamor_21_settings_*.png and glamor_21_settings_dom*.json.
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

const shot = (name) => path.join(SHOTS, `glamor_21_settings_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Boot helpers
// ---------------------------------------------------------------
async function bootDark(vw, vh) {
  const ctx = await browser.newContext({ viewport: { width: vw, height: vh || 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => errors.push(`[dark ${vw}] ${e.message}`));
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
  return { ctx, page };
}

async function bootLight(vw, vh) {
  const ctx = await browser.newContext({ viewport: { width: vw, height: vh || 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => errors.push(`[light ${vw}] ${e.message}`));
  // Navigate first, then set localStorage (canonical recipe from G4+)
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    let p = {};
    try { p = JSON.parse(raw || "{}"); } catch (_) {}
    p.theme = "light";
    delete p.viewAsMember;
    localStorage.setItem("cashflux:prefs", JSON.stringify(p));
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  // Wait for light theme to apply
  await page.waitForFunction(() => document.documentElement.getAttribute("data-theme") === "light", { timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(600);
  return { ctx, page };
}

// ---------------------------------------------------------------
// Open the Settings panel via the household card button (.hh)
// ---------------------------------------------------------------
async function openSettings(page) {
  // The HouseholdCard is the button with class "hh" at the rail bottom
  const hhBtn = page.locator("button.hh").first();
  if (await hhBtn.count() > 0) {
    await hhBtn.click();
  } else {
    // Fallback: any button with title containing "Settings"
    const fallback = page.locator('button[title*="Settings"]').first();
    if (await fallback.count() > 0) {
      await fallback.click();
    } else {
      // Fallback 2: gear icon button
      const gear = page.locator('button[aria-label*="settings" i], button[title*="settings" i]').first();
      if (await gear.count() > 0) await gear.click();
    }
  }
  // Wait for the FlipPanel to appear
  await page.waitForSelector('.flip-panel, [class*="flip"], .settings-panel, .panel-back, .set-label', { timeout: 15000 });
  await page.waitForTimeout(800);
}

// ---------------------------------------------------------------
// DOM audit
// ---------------------------------------------------------------
async function auditDOM(page) {
  return page.evaluate(() => {
    // Panel detection
    const panel = document.querySelector('.flip-panel, [class*="flip"], [data-testid="settings-panel"]');
    const panelFound = !!panel;

    // Section labels (set-label)
    const setLabels = [...document.querySelectorAll(".set-label")];
    const sectionNames = setLabels.map(l => l.textContent.trim());

    // Buttons
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim()).filter(t => t.length > 0);

    // All inputs
    const inputs = [...document.querySelectorAll("input")];
    const inputTypes = inputs.map(i => i.type);
    const inputCount = inputs.length;

    // Password inputs (AI key, web search key, backend token)
    const passwordInputs = inputs.filter(i => i.type === "password");
    const passwordCount = passwordInputs.length;

    // Selects
    const selects = [...document.querySelectorAll("select")];
    const selectCount = selects.length;
    const selectAria = selects.map(s => s.getAttribute("aria-label") || "(no aria-label)");

    // Labels
    const labels = [...document.querySelectorAll("label")];
    const labelCount = labels.length;

    // Toggle rows
    const toggleRows = [...document.querySelectorAll(".toggle-row")];
    const toggleCount = toggleRows.length;
    const toggleLabels = toggleRows.map(r => {
      const s = r.querySelector("span");
      return s ? s.textContent.trim() : "(no span)";
    });

    // Data buttons
    const dataBtns = [...document.querySelectorAll(".data-btn")];
    const dataBtnTexts = dataBtns.map(b => b.textContent.trim());

    // Import buttons (how many?)
    const importBtns = btns.filter(b => /import/i.test(b.textContent));
    const importBtnCount = importBtns.length;

    // Swatch pickers
    const swatches = [...document.querySelectorAll(".swatch, [class*='swatch']")];
    const swatchCount = swatches.length;

    // Heading levels in the panel area
    const headings = [...document.querySelectorAll("h1,h2,h3,h4,h5")];
    const headingInfo = headings.map(h => ({ tag: h.tagName, text: h.textContent.trim().slice(0, 60) }));

    // Member chips
    const memberChips = [...document.querySelectorAll(".member-chip")];
    const memberCount = memberChips.length;

    // Overflow
    const overflows = [...document.querySelectorAll("*")].filter(el => {
      return el.scrollWidth > el.clientWidth + 2;
    });
    const overflowCount = overflows.length;

    // Two-column grid check
    const gridCols2 = [...document.querySelectorAll('[class*="grid-cols-2"], [class*="gridCols2"]')];
    const hasGrid = gridCols2.length > 0;

    // FX rate rows
    const rateRows = [...document.querySelectorAll(".rate-row")];
    const rateRowCount = rateRows.length;

    // Freshness rows
    const freshnessRows = rateRows.length; // same class

    // Theme segmented
    const segmented = [...document.querySelectorAll(".segmented, [class*='segmented']")];
    const segmentedCount = segmented.length;

    // Panel width / dimensions
    const panelEl = document.querySelector('.flip-panel-back, .flip-panel, [class*="flip"]');
    const panelRect = panelEl ? panelEl.getBoundingClientRect() : null;

    return {
      panelFound,
      sectionNames,
      btnTexts: btnTexts.slice(0, 50),
      inputCount, inputTypes: inputTypes.slice(0, 20),
      passwordCount,
      selectCount, selectAria,
      labelCount,
      toggleCount, toggleLabels: toggleLabels.slice(0, 20),
      dataBtnTexts,
      importBtnCount,
      swatchCount,
      headingInfo,
      memberCount,
      overflowCount,
      hasGrid,
      rateRowCount,
      segmentedCount,
      panelDimensions: panelRect ? { width: Math.round(panelRect.width), height: Math.round(panelRect.height) } : null,
    };
  });
}

// ---------------------------------------------------------------
// Light-mode contrast audit
// ---------------------------------------------------------------
async function auditLightContrast(page) {
  return page.evaluate(() => {
    const cs = getComputedStyle(document.documentElement);
    const bodyBg = cs.getPropertyValue("--bg").trim() || getComputedStyle(document.body).backgroundColor;
    const fg = cs.getPropertyValue("--fg").trim() || cs.getPropertyValue("--text").trim();

    // Panel background
    const panelEl = document.querySelector('.flip-panel-back, .flip-panel, [class*="flip"], .set-label')?.closest('div') || document.body;
    const panelBg = getComputedStyle(panelEl).backgroundColor;

    // Set-label color
    const setLabel = document.querySelector(".set-label");
    const setLabelColor = setLabel ? getComputedStyle(setLabel).color : "n/a";
    const setLabelBg = setLabel ? getComputedStyle(setLabel).backgroundColor : "n/a";

    // Toggle row label color
    const toggleRow = document.querySelector(".toggle-row span");
    const toggleColor = toggleRow ? getComputedStyle(toggleRow).color : "n/a";

    // Data button styles
    const dataBtn = document.querySelector(".data-btn");
    const dataBtnColor = dataBtn ? getComputedStyle(dataBtn).color : "n/a";
    const dataBtnBg = dataBtn ? getComputedStyle(dataBtn).backgroundColor : "n/a";
    const dataBtnBorder = dataBtn ? getComputedStyle(dataBtn).borderColor : "n/a";

    // Primary button (if any)
    const primaryBtn = document.querySelector(".btn-primary");
    const primaryBtnColor = primaryBtn ? getComputedStyle(primaryBtn).color : "n/a";
    const primaryBtnBg = primaryBtn ? getComputedStyle(primaryBtn).backgroundColor : "n/a";

    // Muted text
    const muted = document.querySelector(".muted, [class*='faint']");
    const mutedColor = muted ? getComputedStyle(muted).color : "n/a";
    const mutedBg = muted ? getComputedStyle(muted.parentElement || muted).backgroundColor : "n/a";

    // Input field
    const inputEl = document.querySelector("input.set-input, input[class*='set-input']");
    const inputColor = inputEl ? getComputedStyle(inputEl).color : "n/a";
    const inputBg = inputEl ? getComputedStyle(inputEl).backgroundColor : "n/a";

    // Select field
    const selectEl = document.querySelector("select.set-input");
    const selectColor = selectEl ? getComputedStyle(selectEl).color : "n/a";
    const selectBg = selectEl ? getComputedStyle(selectEl).backgroundColor : "n/a";

    // Main content bg
    const mainEl = document.querySelector("main, .main-content, [role='main']");
    const mainBg = mainEl ? getComputedStyle(mainEl).backgroundColor : "n/a";

    // Page bg
    const pageBg = getComputedStyle(document.body).backgroundColor;

    // Data theme
    const dataTheme = document.documentElement.getAttribute("data-theme");

    return {
      dataTheme, pageBg, mainBg, panelBg, bodyBg, fg,
      setLabelColor, setLabelBg,
      toggleColor,
      dataBtnColor, dataBtnBg, dataBtnBorder,
      primaryBtnColor, primaryBtnBg,
      mutedColor, mutedBg,
      inputColor, inputBg,
      selectColor, selectBg,
    };
  });
}

// ===============================================================
// DARK — 1280
// ===============================================================
console.log("--- Dark 1280 ---");
{
  const { ctx, page } = await bootDark(1280);
  await openSettings(page);
  await page.screenshot({ path: shot("dark_1280_top"), fullPage: false });
  await page.screenshot({ path: shot("dark_1280_full"), fullPage: true });

  const dom = await auditDOM(page);
  fs.writeFileSync(path.join(SHOTS, "glamor_21_settings_dom.json"), JSON.stringify(dom, null, 2));
  console.log("panelFound:", dom.panelFound);
  console.log("sectionNames:", dom.sectionNames);
  console.log("inputCount:", dom.inputCount, "passwordCount:", dom.passwordCount);
  console.log("selectCount:", dom.selectCount);
  console.log("labelCount:", dom.labelCount);
  console.log("toggleCount:", dom.toggleCount);
  console.log("dataBtnTexts:", dom.dataBtnTexts);
  console.log("importBtnCount:", dom.importBtnCount);
  console.log("swatchCount:", dom.swatchCount);
  console.log("memberCount:", dom.memberCount);
  console.log("overflowCount:", dom.overflowCount);
  console.log("hasGrid:", dom.hasGrid, "segmentedCount:", dom.segmentedCount);
  console.log("panelDimensions:", dom.panelDimensions);

  // Scroll panel partway to capture middle section (set-body is the scrollable)
  await page.evaluate(() => {
    const body = document.querySelector(".set-body");
    if (body) body.scrollTop = 500;
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: shot("dark_1280_mid"), fullPage: false });

  // Scroll to bottom
  await page.evaluate(() => {
    const body = document.querySelector(".set-body");
    if (body) body.scrollTop = 9999;
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: shot("dark_1280_bottom"), fullPage: false });

  await ctx.close();
}

// ===============================================================
// DARK — 1440
// ===============================================================
console.log("--- Dark 1440 ---");
{
  const { ctx, page } = await bootDark(1440);
  await openSettings(page);
  await page.screenshot({ path: shot("dark_1440"), fullPage: false });
  await page.screenshot({ path: shot("dark_1440_full"), fullPage: true });
  await ctx.close();
}

// ===============================================================
// DARK — 768
// ===============================================================
console.log("--- Dark 768 ---");
{
  const { ctx, page } = await bootDark(768);
  await openSettings(page);
  await page.screenshot({ path: shot("dark_768"), fullPage: false });
  await page.screenshot({ path: shot("dark_768_full"), fullPage: true });
  await ctx.close();
}

// ===============================================================
// LIGHT — 1280
// ===============================================================
console.log("--- Light 1280 ---");
{
  const { ctx, page } = await bootLight(1280);
  // Confirm light
  const theme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  console.log("light theme attr:", theme);
  await openSettings(page);
  await page.screenshot({ path: shot("light_1280_top"), fullPage: false });
  await page.screenshot({ path: shot("light_1280_full"), fullPage: true });

  const contrast = await auditLightContrast(page);
  fs.writeFileSync(path.join(SHOTS, "glamor_21_settings_light_contrast.json"), JSON.stringify(contrast, null, 2));
  console.log("contrast:", JSON.stringify(contrast, null, 2));

  // Mid section
  await page.evaluate(() => {
    const body = document.querySelector(".set-body");
    if (body) body.scrollTop = 500;
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: shot("light_1280_mid"), fullPage: false });

  // Bottom
  await page.evaluate(() => {
    const body = document.querySelector(".set-body");
    if (body) body.scrollTop = 9999;
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: shot("light_1280_bottom"), fullPage: false });

  await ctx.close();
}

// ===============================================================
// LIGHT — 1440
// ===============================================================
console.log("--- Light 1440 ---");
{
  const { ctx, page } = await bootLight(1440);
  await openSettings(page);
  await page.screenshot({ path: shot("light_1440"), fullPage: false });
  await page.screenshot({ path: shot("light_1440_full"), fullPage: true });
  await ctx.close();
}

// ===============================================================
// LIGHT — 768
// ===============================================================
console.log("--- Light 768 ---");
{
  const { ctx, page } = await bootLight(768);
  await openSettings(page);
  await page.screenshot({ path: shot("light_768"), fullPage: false });
  await page.screenshot({ path: shot("light_768_full"), fullPage: true });

  // DOM audit at 768
  const dom768 = await auditDOM(page);
  fs.writeFileSync(path.join(SHOTS, "glamor_21_settings_dom_768.json"), JSON.stringify(dom768, null, 2));
  console.log("768 overflow:", dom768.overflowCount, "panelDims:", dom768.panelDimensions);

  await ctx.close();
}

// ---------------------------------------------------------------
// Summary
// ---------------------------------------------------------------
console.log("\n=== DONE ===");
console.log("Errors:", errors.length ? errors : "none");

const shots = fs.readdirSync(SHOTS).filter(f => f.startsWith("glamor_21_"));
console.log("Screenshots:", shots.length, shots);

await browser.close();
process.exit(errors.length > 0 ? 1 : 0);
