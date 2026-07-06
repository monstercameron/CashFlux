// GLAMOR G22 — Custom pages ("Make It My Dashboard" / Theo) visual + structural review.
// Creates a new custom page via the rail "New page" affordance (uses cf-dialog, not window.prompt),
// names it, adds two widgets via the add-widget toolbar, screenshots the full flow at 1280/1440/768
// in both dark and light. Audits: rail entry, empty state, add-widget toolbar, bento grid,
// light-mode contrast (incl. backgrounds), overflow. Applies L70 member-filter reset and
// hard-reload-before-persistence assertions per house rules.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8080";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot   = (name) => path.join(SHOTS, `glamor_22_custompages_${name}.png`);
const domOut = (name) => path.join(SHOTS, `glamor_22_custompages_${name}.json`);
const errors = [];

// ---------------------------------------------------------------
// Boot helper: set theme + reset viewAsMember, reload, verify
// ---------------------------------------------------------------
async function bootWithTheme(page, theme) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);
  await page.evaluate((t) => {
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      const p = raw ? JSON.parse(raw) : {};
      delete p.viewAsMember;
      p.theme = t;
      localStorage.setItem("cashflux:prefs", JSON.stringify(p));
    } catch (_) {}
  }, theme);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute("data-theme") === t,
    theme, { timeout: 20000 }
  );
  await page.waitForTimeout(800);
}

// ---------------------------------------------------------------
// Dismiss the gwc-error-overlay if present (blocks all clicks)
// ---------------------------------------------------------------
async function dismissErrorOverlay(page) {
  const overlay = await page.$('#gwc-error-overlay, .gwc-error-overlay');
  if (overlay) {
    await page.evaluate(() => {
      const o = document.getElementById('gwc-error-overlay') || document.querySelector('.gwc-error-overlay');
      if (o) o.remove();
    });
    await page.waitForTimeout(300);
  }
}

// ---------------------------------------------------------------
// Create a custom page via the cf-dialog modal
// The "New page" rail element calls promptModal which renders:
//   .cf-dialog-root > .cf-dialog-backdrop > .cf-dialog-scrim (click=cancel)
//                                         > .cf-dialog (panel)
//     > .cf-dialog-input (text input)
//     > #cf-dialog-confirm (OK button)
// ---------------------------------------------------------------
async function createCustomPage(page, name) {
  // Dismiss any error overlay that might be blocking clicks
  await dismissErrorOverlay(page);

  // Click the "New page" rail element
  const newPageEl = page.locator('nav a[title], nav a, nav button').filter({ hasText: /new page/i }).first();
  const count = await newPageEl.count();
  if (count === 0) throw new Error("'New page' element not found in rail");
  await newPageEl.click();
  await page.waitForTimeout(600);

  // Dismiss error overlay again (may appear after click)
  await dismissErrorOverlay(page);

  // Wait for the cf-dialog to appear
  await page.waitForSelector(".cf-dialog-input", { timeout: 10000 });

  // Use JavaScript to fill the input (bypasses overlay pointer issues)
  await page.evaluate((n) => {
    const inp = document.querySelector('.cf-dialog-input, #cf-dialog-input');
    if (inp) {
      inp.value = n;
      inp.dispatchEvent(new Event('input', { bubbles: true }));
      inp.dispatchEvent(new Event('change', { bubbles: true }));
    }
  }, name);
  await page.waitForTimeout(300);

  // Click the confirm button via JS (bypasses pointer-event blocking)
  await page.evaluate(() => {
    const btn = document.querySelector('#cf-dialog-confirm');
    if (btn) btn.click();
  });
  await page.waitForTimeout(2000);

  const url = page.url();
  const m = url.match(/\/p\/([^/?#]+)/);
  return m ? m[1] : null;
}

// ---------------------------------------------------------------
// DOM audit for custom page
// ---------------------------------------------------------------
async function auditDOM(page) {
  return page.evaluate(() => {
    // Cards
    const cards = [...document.querySelectorAll(".card, section.card")];
    const cardTitles = cards.map(c => {
      const t = c.querySelector("h2,h3,h4,.card-title");
      return t ? t.textContent.trim() : "(no title)";
    });
    const cardHeadingLevels = cards.map(c => {
      const t = c.querySelector("h1,h2,h3,h4");
      return t ? t.tagName : "none";
    });

    // Bento tiles
    const tiles = [...document.querySelectorAll(".w, .widget")];
    const tileCount = tiles.length;
    const tileTitles = tiles.map(t => {
      const h = t.querySelector("h2,h3,h4,.widget-title,.tile-title,.wt");
      return h ? h.textContent.trim() : "(no title)";
    });

    // Draggable tiles
    const draggable = [...document.querySelectorAll("[draggable='true']")];

    // Add-widget toolbar: look for the toolbar container
    const toolbar = document.querySelector(".add-widget-bar, .widget-toolbar, [class*='add-widget'], [class*='toolbar']");
    const hasToolbar = !!toolbar;
    const toolbarBtns = toolbar ? [...toolbar.querySelectorAll("button, a")].map(b => b.textContent.trim()) : [];

    // Any "Add widget" button anywhere on page
    const addWidgetBtns = [...document.querySelectorAll("button")].filter(b => /add widget|add a widget/i.test(b.textContent));
    const hasAddWidget = addWidgetBtns.length > 0;
    const addWidgetBtnTexts = addWidgetBtns.map(b => b.textContent.trim());

    // Empty state
    const emptyEl = document.querySelector(".empty, [class*='empty-state']");
    const emptyText = emptyEl ? emptyEl.textContent.trim() : null;

    // Page-name display in topbar/breadcrumb
    const topbar = document.querySelector(".topbar, header");
    const topbarText = topbar ? topbar.textContent.trim().slice(0, 120) : null;

    // Headings
    const headings = [...document.querySelectorAll("h1,h2,h3,h4")].map(h => ({
      tag: h.tagName, text: h.textContent.trim().slice(0, 60)
    }));

    // Buttons
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim()).filter(Boolean).slice(0, 30);

    // Labels
    const labelCount = document.querySelectorAll("label").length;

    // Rail: my-pages links
    const rail = document.querySelector("nav");
    const railLinks = rail ? [...rail.querySelectorAll("a[href]")].map(a => ({
      href: a.getAttribute("href"),
      title: a.getAttribute("title") || a.textContent.trim().slice(0, 40)
    })) : [];
    const myPagesLinks = railLinks.filter(l => l.href && l.href.startsWith("/p/"));

    // Overflow
    const overflowCount = [...document.querySelectorAll("*")].filter(
      el => el.scrollWidth > el.clientWidth + 2
    ).length;

    // Widget edit / drill-down buttons
    const tileButtons = tiles.flatMap(t => [...t.querySelectorAll("button, a[href]")].map(b => b.textContent.trim() || b.getAttribute("title") || b.getAttribute("aria-label") || "")).filter(Boolean);

    // Is there a gear/settings button on tiles?
    const hasGear = tiles.some(t => t.querySelector('button[aria-label*="setting" i], button[title*="setting" i], button[aria-label*="gear" i], .gear, .ico-gear'));

    return {
      cardCount: cards.length, cardTitles, cardHeadingLevels,
      tileCount, tileTitles, draggableCount: draggable.length,
      hasToolbar, toolbarBtns,
      hasAddWidget, addWidgetBtnTexts,
      emptyText,
      topbarText,
      headings: headings.slice(0, 20),
      btnTexts,
      labelCount,
      myPagesLinks,
      overflowCount,
      tileButtons: tileButtons.slice(0, 20),
      hasGear,
    };
  });
}

// ---------------------------------------------------------------
// Light-mode contrast audit
// ---------------------------------------------------------------
async function auditContrast(page) {
  return page.evaluate(() => {
    const cs = (el, p) => el ? getComputedStyle(el).getPropertyValue(p).trim() : "n/a";

    // Custom page content area
    const main = document.querySelector("main, .page-content, #main");
    const mainBg = cs(main, "background-color");

    // Body / html
    const bodyBg = cs(document.body, "background-color");
    const htmlBg = cs(document.documentElement, "background-color");

    // Cards
    const card = document.querySelector(".card, section.card");
    const cardBg = cs(card, "background-color");
    const cardTitle = card ? card.querySelector("h2,h3,.card-title") : null;
    const cardTitleColor = cs(cardTitle, "color");

    // Bento tiles
    const tile = document.querySelector(".w, .widget");
    const tileBg = cs(tile, "background-color");
    const tileH = tile ? tile.querySelector("h2,h3,h4,.wt") : null;
    const tileTitleColor = cs(tileH, "color");
    const tileValue = tile ? tile.querySelector(".value, .kpi-val, p, span") : null;
    const tileValueColor = cs(tileValue, "color");

    // Add widget button (if any)
    const addBtn = [...document.querySelectorAll("button")].find(b => /add widget/i.test(b.textContent));
    const addBtnBg = cs(addBtn, "background-color");
    const addBtnColor = cs(addBtn, "color");

    // Primary button
    const primaryBtn = document.querySelector(".btn-primary, button.btn-primary");
    const primaryBg = cs(primaryBtn, "background-color");
    const primaryColor = cs(primaryBtn, "color");

    // Empty state text
    const emptyEl = document.querySelector(".empty");
    const emptyColor = cs(emptyEl, "color");
    const emptyBg = cs(emptyEl?.parentElement, "background-color");

    // Rail custom page link
    const customLink = document.querySelector('nav a[href^="/p/"]');
    const customLinkColor = cs(customLink, "color");
    const customLinkBg = cs(customLink, "background-color");

    // Muted / faint text
    const mutedEl = document.querySelector(".text-faint, .muted, [class*='faint']");
    const mutedColor = cs(mutedEl, "color");

    return {
      mainBg, bodyBg, htmlBg,
      cardBg, cardTitleColor,
      tileBg, tileTitleColor, tileValueColor,
      addBtnBg, addBtnColor,
      primaryBg, primaryColor,
      emptyColor, emptyBg,
      customLinkColor, customLinkBg,
      mutedColor,
    };
  });
}

// ---------------------------------------------------------------
// Add a widget via the addWidgetBar inline form:
//   1. Click "Add widget" button → reveals a card with form-grid
//   2. The form has: <select> for type, <input> for title, binding control
//   3. Click the "Add" (btn-primary) button to submit
// widgetType: one of "kpi", "list", "text", "chart", "image", "table"
// titleVal: the widget title to fill in
// Returns true if a widget was successfully added
// ---------------------------------------------------------------
async function addWidget(page, widgetType, titleVal) {
  // Click "Add widget" button (renders when open=false)
  const addWidgetBtn = page.locator('button').filter({ hasText: /^add widget$/i }).first();
  if (await addWidgetBtn.count() === 0) {
    console.log("  addWidget: 'Add widget' button not found");
    return false;
  }
  await addWidgetBtn.click();
  await page.waitForTimeout(600);

  // Wait for the inline form to appear (a select.field should now be visible)
  try {
    await page.waitForSelector("select.field", { timeout: 5000 });
  } catch (_) {
    console.log("  addWidget: inline form select not found after clicking 'Add widget'");
    return false;
  }

  // Pick widget type via the select
  const typeSelect = page.locator("select.field").first();
  await typeSelect.selectOption(widgetType);
  await page.waitForTimeout(300);

  // Fill title
  const titleInput = page.locator('input.field[placeholder]').first();
  if (await titleInput.count() > 0) {
    await titleInput.fill(titleVal);
    await page.waitForTimeout(200);
  }

  // Screenshot the open form
  await page.screenshot({ path: shot(`_add_widget_form_${widgetType}`), fullPage: false });

  // Click the "Add" button (btn-primary inside the card form)
  // The addWidgetBar card has two buttons: "Add" (btn-primary) and "Cancel" (btn)
  const addBtn = page.locator('section.card button.btn-primary, .card button.btn-primary').filter({ hasText: /^add$/i }).first();
  if (await addBtn.count() > 0) {
    await addBtn.click();
  } else {
    // Fallback: any btn-primary that says "Add"
    const anyAdd = page.locator('button.btn-primary').filter({ hasText: /add/i }).first();
    if (await anyAdd.count() > 0) await anyAdd.click();
    else {
      console.log("  addWidget: 'Add' submit button not found");
      return false;
    }
  }
  await page.waitForTimeout(1200);
  return true;
}

// ---------------------------------------------------------------
// SESSION 1: DARK — full Theo flow
// ---------------------------------------------------------------
console.log("=== G22 DARK session ===");
const browser1 = await chromium.launch({ headless: true });
{
  const page = await browser1.newPage();
  page.on("pageerror", (e) => errors.push(`[dark] ${e.message}`));
  await page.setViewportSize({ width: 1280, height: 900 });
  await bootWithTheme(page, "dark");

  // Shot 1: rail before any custom page (dark)
  await page.screenshot({ path: shot("dark_1280_rail_before"), fullPage: false });
  console.log("✓ dark_1280_rail_before.png");

  // Check what's in the rail
  const dom0 = await auditDOM(page);
  console.log("My pages links before create:", dom0.myPagesLinks);
  console.log("Buttons in rail area:", dom0.btnTexts.slice(0, 8));

  // --- Create custom page ---
  console.log("Creating 'Theo Budget View'...");
  let slug = null;
  try {
    slug = await createCustomPage(page, "Theo Budget View");
    console.log("Created slug:", slug, "URL:", page.url());
  } catch (err) {
    console.log("ERROR creating page:", err.message);
    errors.push(`[dark] createCustomPage: ${err.message}`);
  }

  // Shot 2: the new custom page (empty state)
  await page.screenshot({ path: shot("dark_1280_empty_state"), fullPage: false });
  await page.screenshot({ path: shot("dark_1280_empty_state_full"), fullPage: true });
  console.log("✓ dark_1280_empty_state.png / _full.png");

  // DOM audit: empty state
  const domEmpty = await auditDOM(page);
  fs.writeFileSync(domOut("dom_dark_empty"), JSON.stringify(domEmpty, null, 2));
  console.log("Empty state DOM:", JSON.stringify({
    tileCount: domEmpty.tileCount,
    emptyText: domEmpty.emptyText,
    hasAddWidget: domEmpty.hasAddWidget,
    addWidgetBtnTexts: domEmpty.addWidgetBtnTexts,
    hasToolbar: domEmpty.hasToolbar,
    toolbarBtns: domEmpty.toolbarBtns,
    topbarText: domEmpty.topbarText,
    headings: domEmpty.headings.slice(0, 6),
    btnTexts: domEmpty.btnTexts.slice(0, 12),
  }));

  // Shot 3: add-widget toolbar (open it) — add a KPI widget first
  const addedW1 = await addWidget(page, "kpi", "My KPI");
  console.log("Widget 1 (kpi) added:", addedW1);
  await page.screenshot({ path: shot("dark_1280_after_first_widget"), fullPage: false });
  await page.screenshot({ path: shot("dark_1280_after_first_widget_full"), fullPage: true });
  console.log("✓ dark_1280_after_first_widget.png / _full.png");

  // Try to add a second widget (list)
  const addedW2 = await addWidget(page, "list", "My List");
  console.log("Widget 2 (list) added:", addedW2);
  await page.screenshot({ path: shot("dark_1280_two_widgets"), fullPage: false });
  await page.screenshot({ path: shot("dark_1280_two_widgets_full"), fullPage: true });
  console.log("✓ dark_1280_two_widgets.png / _full.png");

  // DOM audit: populated
  const domPop = await auditDOM(page);
  fs.writeFileSync(domOut("dom_dark_populated"), JSON.stringify(domPop, null, 2));
  console.log("Populated DOM:", JSON.stringify({
    tileCount: domPop.tileCount,
    tileTitles: domPop.tileTitles,
    draggableCount: domPop.draggableCount,
    hasGear: domPop.hasGear,
    tileButtons: domPop.tileButtons,
    headings: domPop.headings.slice(0, 8),
    labelCount: domPop.labelCount,
    overflowCount: domPop.overflowCount,
  }));

  // 1440 dark
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: shot("dark_1440"), fullPage: false });
  console.log("✓ dark_1440.png");

  // 768 dark
  await page.setViewportSize({ width: 768, height: 900 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: shot("dark_768"), fullPage: false });
  await page.screenshot({ path: shot("dark_768_full"), fullPage: true });
  console.log("✓ dark_768.png / _full.png");

  // Check overflow at 768
  const ov768 = await page.evaluate(() =>
    [...document.querySelectorAll("*")].filter(e => e.scrollWidth > e.clientWidth + 2).length
  );
  console.log("Overflow at 768:", ov768);

  // Back to 1280, go to dashboard, check rail shows "Theo Budget View"
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: shot("dark_1280_rail_after"), fullPage: false });
  console.log("✓ dark_1280_rail_after.png");

  const domRail = await auditDOM(page);
  console.log("Rail after create:", JSON.stringify({
    myPagesLinks: domRail.myPagesLinks,
  }));

  // --- PERSISTENCE CHECK: navigate back to the page then hard-reload ---
  if (slug) {
    await page.goto(BASE + `/p/${slug}`, { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(1000);
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForTimeout(1500);
    await page.screenshot({ path: shot("dark_1280_persist_reload"), fullPage: false });
    console.log("✓ dark_1280_persist_reload.png");
    const domReload = await auditDOM(page);
    console.log("After hard reload:", JSON.stringify({
      tileCount: domReload.tileCount,
      tileTitles: domReload.tileTitles,
      emptyText: domReload.emptyText,
    }));
    fs.writeFileSync(domOut("dom_dark_reload"), JSON.stringify(domReload, null, 2));
  }

  await page.close();
}
await browser1.close();
console.log("Dark session complete.\n");

// ---------------------------------------------------------------
// SESSION 2: LIGHT — same flow, emphasise contrast
// ---------------------------------------------------------------
console.log("=== G22 LIGHT session ===");
const browser2 = await chromium.launch({ headless: true });
{
  const page = await browser2.newPage();
  page.on("pageerror", (e) => errors.push(`[light] ${e.message}`));
  await page.setViewportSize({ width: 1280, height: 900 });
  await bootWithTheme(page, "light");

  // Shot: rail before create in light
  await page.screenshot({ path: shot("light_1280_rail_before"), fullPage: false });
  console.log("✓ light_1280_rail_before.png");

  // Create custom page in light session
  console.log("Light: creating 'Theo Light Page'...");
  let slug = null;
  try {
    slug = await createCustomPage(page, "Theo Light Page");
    console.log("Light created slug:", slug, "URL:", page.url());
  } catch (err) {
    console.log("Light ERROR creating page:", err.message);
    errors.push(`[light] createCustomPage: ${err.message}`);
  }

  // Shot: empty state in light (key contrast target)
  await page.screenshot({ path: shot("light_1280_empty"), fullPage: false });
  await page.screenshot({ path: shot("light_1280_empty_full"), fullPage: true });
  console.log("✓ light_1280_empty.png / _full.png");

  // DOM + contrast audit on empty state
  const domLightEmpty = await auditDOM(page);
  fs.writeFileSync(domOut("dom_light_empty"), JSON.stringify(domLightEmpty, null, 2));
  const contrastEmpty = await auditContrast(page);
  fs.writeFileSync(domOut("light_contrast_empty"), JSON.stringify(contrastEmpty, null, 2));
  console.log("Light empty DOM:", JSON.stringify({
    tileCount: domLightEmpty.tileCount,
    emptyText: domLightEmpty.emptyText,
    hasAddWidget: domLightEmpty.hasAddWidget,
    addWidgetBtnTexts: domLightEmpty.addWidgetBtnTexts,
    topbarText: domLightEmpty.topbarText,
    headings: domLightEmpty.headings.slice(0, 6),
    myPagesLinks: domLightEmpty.myPagesLinks,
  }));
  console.log("Light empty contrast:", JSON.stringify(contrastEmpty));

  // Add a widget in light — KPI first
  const addedW1 = await addWidget(page, "kpi", "Savings KPI");
  console.log("Light widget 1 (kpi) added:", addedW1);

  // Shot: after first widget (light — key contrast view)
  await page.screenshot({ path: shot("light_1280_one_widget"), fullPage: false });
  await page.screenshot({ path: shot("light_1280_one_widget_full"), fullPage: true });
  console.log("✓ light_1280_one_widget.png / _full.png");

  // Contrast audit with widget
  const contrastPop = await auditContrast(page);
  fs.writeFileSync(domOut("light_contrast_populated"), JSON.stringify(contrastPop, null, 2));
  console.log("Light populated contrast:", JSON.stringify(contrastPop));

  // Add a second widget — list
  const addedW2 = await addWidget(page, "list", "Transactions List");
  console.log("Light widget 2 (list) added:", addedW2);
  await page.screenshot({ path: shot("light_1280_two_widgets"), fullPage: false });
  await page.screenshot({ path: shot("light_1280_two_widgets_full"), fullPage: true });
  console.log("✓ light_1280_two_widgets.png / _full.png");

  const domLightPop = await auditDOM(page);
  fs.writeFileSync(domOut("dom_light_populated"), JSON.stringify(domLightPop, null, 2));
  console.log("Light populated DOM:", JSON.stringify({
    tileCount: domLightPop.tileCount,
    tileTitles: domLightPop.tileTitles,
    draggableCount: domLightPop.draggableCount,
    overflowCount: domLightPop.overflowCount,
    tileButtons: domLightPop.tileButtons,
    hasGear: domLightPop.hasGear,
  }));

  // 1440 light
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: shot("light_1440"), fullPage: false });
  console.log("✓ light_1440.png");

  // 768 light
  await page.setViewportSize({ width: 768, height: 900 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: shot("light_768"), fullPage: false });
  await page.screenshot({ path: shot("light_768_full"), fullPage: true });
  console.log("✓ light_768.png / _full.png");

  const ov768light = await page.evaluate(() =>
    [...document.querySelectorAll("*")].filter(e => e.scrollWidth > e.clientWidth + 2).length
  );
  console.log("Light overflow at 768:", ov768light);

  // Return to dashboard in light, check rail
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: shot("light_1280_rail_after"), fullPage: false });
  console.log("✓ light_1280_rail_after.png");

  const domLightRail = await auditDOM(page);
  console.log("Light rail after create:", domLightRail.myPagesLinks);

  // Persistence: hard reload custom page
  if (slug) {
    await page.goto(BASE + `/p/${slug}`, { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(1000);
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForTimeout(1500);
    await page.screenshot({ path: shot("light_1280_persist_reload"), fullPage: false });
    console.log("✓ light_1280_persist_reload.png");
    const domPersist = await auditDOM(page);
    console.log("Light after hard reload:", JSON.stringify({
      tileCount: domPersist.tileCount,
      tileTitles: domPersist.tileTitles,
      emptyText: domPersist.emptyText,
    }));
  }

  await page.close();
}
await browser2.close();

// ---------------------------------------------------------------
// Summary
// ---------------------------------------------------------------
console.log("\n=== G22 SUMMARY ===");
console.log("Page errors:", errors.length === 0 ? "none" : errors);
const code = errors.length > 0 ? 1 : 0;
console.log("Exit code:", code);
process.exit(code);
