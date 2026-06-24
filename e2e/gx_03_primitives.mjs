// GX3 — Component primitives visual review.
// Story: "A Thousand Small Cuts" — buttons, inputs, selects, tables, badges, tooltips.
//
// Captures screenshots at 1280x800 and 768x1024 in dark and light themes for:
//   • /transactions — table, filter toolbar, chips, add modal (buttons/inputs/selects)
//   • /budgets — badges, progress bars, pills
//   • /goals — pace badges, progress bars
//
// Also measures getComputedStyle on every primitive type, logging as JSON.
// Saves to e2e/screenshots/ with prefix gx03_.
// Exit code 0 — evidence-harvest script, not a pass/fail gate.

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const SHOTS_DIR = path.join(__dirname, "screenshots");
fs.mkdirSync(SHOTS_DIR, { recursive: true });

const WIDTHS = [1280, 768];
const HEIGHTS = { 1280: 800, 768: 1024 };

async function bootWithTheme(browser, width, theme) {
  const ctx = await browser.newContext({
    viewport: { width, height: HEIGHTS[width] },
  });
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });

  await page.evaluate(({ t }) => {
    localStorage.setItem("cashflux:theme", JSON.stringify(t));
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      if (raw) {
        const p = JSON.parse(raw);
        p.theme = t;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      }
    } catch (_) {}
  }, { t: theme });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('aside.rail, nav[aria-label="Main navigation"]', {
    timeout: 60000,
  });
  await page.waitForTimeout(1500); // settle WASM + transitions

  return { page, ctx, errors };
}

async function shot(page, name) {
  const p = path.join(SHOTS_DIR, name);
  await page.screenshot({ path: p, fullPage: false });
  console.log(`  shot: ${name}`);
  return name;
}

async function nav(page, route) {
  await page.evaluate((r) => window.history.pushState({}, "", r), route);
  await page.evaluate(() => window.dispatchEvent(new PopStateEvent("popstate")));
  await page.waitForTimeout(1000);
}

// ── Measurement helpers ────────────────────────────────────────────────────────

async function measureButtons(page) {
  return page.evaluate(() => {
    function measure(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        height: cs.height,
        paddingTop: cs.paddingTop,
        paddingBottom: cs.paddingBottom,
        paddingLeft: cs.paddingLeft,
        paddingRight: cs.paddingRight,
        borderRadius: cs.borderRadius,
        fontSize: cs.fontSize,
        fontWeight: cs.fontWeight,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        border: cs.border,
        text: el.textContent.trim().slice(0, 40),
      };
    }
    return {
      btnPrimary: measure(document.querySelector('.btn.btn-primary, .btn-primary')),
      btn: measure(document.querySelector('.btn:not(.btn-primary):not(.btn-del):not(.btn-ghost-danger):not(.btn-sm)')),
      btnDel: measure(document.querySelector('.btn-del')),
      btnGhostDanger: measure(document.querySelector('.btn-ghost-danger')),
      btnSm: measure(document.querySelector('.btn-sm')),
      btnLink: measure(document.querySelector('.btn-link')),
    };
  });
}

async function measureInputs(page) {
  return page.evaluate(() => {
    function measure(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        height: cs.height,
        minHeight: cs.minHeight,
        paddingTop: cs.paddingTop,
        paddingBottom: cs.paddingBottom,
        paddingLeft: cs.paddingLeft,
        borderRadius: cs.borderRadius,
        borderColor: cs.borderColor,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        fontSize: cs.fontSize,
      };
    }
    const textInput = document.querySelector('input[type="text"], input[type="search"], input:not([type="checkbox"]):not([type="radio"]):not([type="submit"]):not([type="button"]):not([type="color"])');
    const numberInput = document.querySelector('input[type="number"]');
    const sel = document.querySelector('select');
    const textarea = document.querySelector('textarea');

    const fieldClass = document.querySelector('.field:not(select):not(button)');

    return {
      textInput: measure(textInput),
      numberInput: measure(numberInput),
      select: measure(sel),
      fieldClass: measure(fieldClass),
      textarea: measure(textarea),
    };
  });
}

async function measureTable(page) {
  return page.evaluate(() => {
    function measureEl(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        height: cs.height,
        paddingTop: cs.paddingTop,
        paddingBottom: cs.paddingBottom,
        paddingLeft: cs.paddingLeft,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        fontSize: cs.fontSize,
        fontWeight: cs.fontWeight,
        borderBottom: cs.borderBottom,
      };
    }
    const table = document.querySelector('.txn-table');
    const thead = document.querySelector('.txn-table thead tr');
    const th = document.querySelector('.txn-table thead th');
    const tbody1 = document.querySelector('.txn-table tbody tr:nth-child(1)');
    const tbody2 = document.querySelector('.txn-table tbody tr:nth-child(2)');
    const amountCell = document.querySelector('.txn-table .td-amount');
    const thSort = document.querySelector('.txn-table .th-sort');
    const clrToggle = document.querySelector('.txn-table .clr-toggle');

    return {
      tablePresent: !!table,
      th: measureEl(th),
      tr1: measureEl(tbody1),
      tr2: measureEl(tbody2),
      amountCell: measureEl(amountCell),
      thSort: measureEl(thSort),
      clrToggle: measureEl(clrToggle),
      rowCount: document.querySelectorAll('.txn-table tbody tr').length,
    };
  });
}

async function measureFilterChips(page) {
  return page.evaluate(() => {
    function measureEl(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        display: cs.display,
        borderRadius: cs.borderRadius,
        paddingTop: cs.paddingTop,
        paddingBottom: cs.paddingBottom,
        paddingLeft: cs.paddingLeft,
        paddingRight: cs.paddingRight,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        fontSize: cs.fontSize,
        border: cs.border,
      };
    }
    const toolbar = document.querySelector('.filter-toolbar');
    const chip = document.querySelector('.filter-chip');
    const filterBadge = document.querySelector('.filter-badge');
    const chipX = document.querySelector('.chip-x');
    const chipClear = document.querySelector('.chip-clear-all');
    const searchInput = document.querySelector('.filter-search input, .filter-search');

    return {
      toolbarPresent: !!toolbar,
      chip: measureEl(chip),
      filterBadge: measureEl(filterBadge),
      chipX: measureEl(chipX),
      chipClearAll: measureEl(chipClear),
      filterSearch: measureEl(searchInput),
      chipCount: document.querySelectorAll('.filter-chip').length,
    };
  });
}

async function measureBadgesAndPills(page) {
  return page.evaluate(() => {
    function measureEl(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        display: cs.display,
        borderRadius: cs.borderRadius,
        paddingTop: cs.paddingTop,
        paddingBottom: cs.paddingBottom,
        paddingLeft: cs.paddingLeft,
        paddingRight: cs.paddingRight,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        fontSize: cs.fontSize,
        fontWeight: cs.fontWeight,
        border: cs.border,
        text: el.textContent.trim().slice(0, 30),
      };
    }
    return {
      badge: measureEl(document.querySelector('.badge')),
      badgeSoon: measureEl(document.querySelector('.badge-soon')),
      pill: measureEl(document.querySelector('.pill')),
      paceBadge: measureEl(document.querySelector('.pace-badge')),
      paceFinal: measureEl(document.querySelector('.pace-final')),
      paceOverdue: measureEl(document.querySelector('.pace-overdue')),
      paceSoon: measureEl(document.querySelector('.pace-soon')),
      paceOntrack: measureEl(document.querySelector('.pace-ontrack')),
      rankBadge: measureEl(document.querySelector('.rank-badge')),
      prioBadge: measureEl(document.querySelector('.badge-prio')),
      prioHigh: measureEl(document.querySelector('.prio-high')),
      prioMed: measureEl(document.querySelector('.prio-med')),
      prioLow: measureEl(document.querySelector('.prio-low')),
    };
  });
}

async function measureProgressBars(page) {
  return page.evaluate(() => {
    function measureEl(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        height: cs.height,
        borderRadius: cs.borderRadius,
        backgroundColor: cs.backgroundColor,
        border: cs.border,
      };
    }
    const bars = document.querySelectorAll('.bar');
    const fills = document.querySelectorAll('.bar-fill');
    const firstFill = fills[0] || null;
    const nearFill = document.querySelector('.bar-fill.near');
    const overFill = document.querySelector('.bar-fill.over');
    const doneFill = document.querySelector('.bar-fill.done');
    const overdueFill = document.querySelector('.bar-fill.overdue');
    const soonFill = document.querySelector('.bar-fill.soon');
    return {
      barCount: bars.length,
      bar: measureEl(bars[0] || null),
      fillDefault: measureEl(firstFill),
      fillNear: measureEl(nearFill),
      fillOver: measureEl(overFill),
      fillDone: measureEl(doneFill),
      fillOverdue: measureEl(overdueFill),
      fillSoon: measureEl(soonFill),
    };
  });
}

async function measureTooltips(page) {
  return page.evaluate(() => {
    // Check for real tooltip elements vs native title= attributes
    const titledEls = Array.from(document.querySelectorAll('[title]'));
    const tooltipEls = document.querySelectorAll('[role="tooltip"], .tooltip, [data-tooltip]');
    const btnWithTitle = titledEls.filter(el => el.tagName === 'BUTTON');
    const linkWithTitle = titledEls.filter(el => el.tagName === 'A');

    return {
      nativeTitleCount: titledEls.length,
      realTooltipElements: tooltipEls.length,
      buttonsWithTitle: btnWithTitle.length,
      buttonTitleSamples: btnWithTitle.slice(0, 5).map(el => ({
        tag: el.tagName,
        cls: el.className.slice(0, 50),
        title: el.getAttribute('title'),
        ariaLabel: el.getAttribute('aria-label'),
      })),
      linksWithTitle: linkWithTitle.length,
      tooltipImplementation: tooltipEls.length > 0 ? 'real-tooltip-component' : 'native-title-only',
    };
  });
}

async function measureModalButtons(page) {
  return page.evaluate(() => {
    function measureEl(el) {
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      return {
        height: cs.height,
        minHeight: cs.minHeight,
        paddingTop: cs.paddingTop,
        paddingBottom: cs.paddingBottom,
        paddingLeft: cs.paddingLeft,
        borderRadius: cs.borderRadius,
        fontSize: cs.fontSize,
        fontWeight: cs.fontWeight,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        border: cs.border,
        text: el.textContent.trim().slice(0, 40),
      };
    }
    // Modal/flip-panel buttons
    const setBtn = document.querySelector('.set-btn');
    const setBtnSave = document.querySelector('.set-btn.save');
    const setBtnCancel = document.querySelector('.set-btn.cancel');
    const setBtnClose = document.querySelector('.set-btn.close');
    const setBtnPrimary = document.querySelector('.set-btn.primary');

    // Regular form buttons in any modal/card
    const allBtns = Array.from(document.querySelectorAll('.btn'));
    const primaries = Array.from(document.querySelectorAll('.btn.btn-primary'));

    return {
      setBtn: measureEl(setBtn),
      setBtnSave: measureEl(setBtnSave),
      setBtnCancel: measureEl(setBtnCancel),
      setBtnClose: measureEl(setBtnClose),
      setBtnPrimary: measureEl(setBtnPrimary),
      regularBtnCount: allBtns.length,
      primaryBtnCount: primaries.length,
    };
  });
}

// ── Main ──────────────────────────────────────────────────────────────────────

const browser = await chromium.launch({ headless: true });
const report = { screenshots: [], measurements: {} };

for (const theme of ["dark", "light"]) {
  for (const width of WIDTHS) {
    const label = `${theme}_${width}`;
    console.log(`\n=== ${label} ===`);

    const { page, ctx, errors } = await bootWithTheme(browser, width, theme);

    // ── /transactions ──────────────────────────────────────────────────────
    console.log(`\n  -- /transactions (${label}) --`);
    await nav(page, "/transactions");
    await page.waitForTimeout(800);

    const txnShot = `gx03_transactions_${label}.png`;
    await shot(page, txnShot);
    report.screenshots.push(txnShot);

    report.measurements[`transactions_table_${label}`] = await measureTable(page);
    report.measurements[`transactions_buttons_${label}`] = await measureButtons(page);
    report.measurements[`transactions_inputs_${label}`] = await measureInputs(page);
    report.measurements[`transactions_filter_chips_${label}`] = await measureFilterChips(page);
    report.measurements[`transactions_tooltips_${label}`] = await measureTooltips(page);
    console.log("  table:", JSON.stringify(report.measurements[`transactions_table_${label}`].th));
    console.log("  btns:", JSON.stringify(report.measurements[`transactions_buttons_${label}`].btn));
    console.log("  chips:", JSON.stringify(report.measurements[`transactions_filter_chips_${label}`].chip));

    // ── Try to open Add Transaction modal ──────────────────────────────────
    console.log(`\n  -- Add Transaction modal (${label}) --`);
    try {
      // Try clicking a visible "Add" or "+ Add" button that opens the transaction modal
      const addBtns = await page.$$('.btn.btn-primary, button:has-text("Add"), button:has-text("New")');
      let opened = false;
      for (const btn of addBtns.slice(0, 3)) {
        try {
          await btn.click({ timeout: 2000 });
          await page.waitForTimeout(600);
          const panel = await page.$('.set-body, .flip-face, [class*="modal"], [role="dialog"]');
          if (panel) { opened = true; break; }
        } catch (_) {}
      }
      if (!opened) {
        // Try the +Add menu in the topbar
        try {
          await page.click('.add-btn', { timeout: 2000 });
          await page.waitForTimeout(400);
          // Click "New transaction" menu item
          await page.click('.add-item:has-text("transaction"), .add-item:first-child', { timeout: 2000 });
          await page.waitForTimeout(800);
          opened = true;
        } catch (_) {}
      }
      const modalShot = `gx03_modal_${label}.png`;
      await shot(page, modalShot);
      report.screenshots.push(modalShot);

      report.measurements[`modal_buttons_${label}`] = await measureModalButtons(page);
      report.measurements[`modal_inputs_${label}`] = await measureInputs(page);
      console.log("  modal set-btn.save:", JSON.stringify(report.measurements[`modal_buttons_${label}`].setBtnSave));
      console.log("  modal inputs:", JSON.stringify(report.measurements[`modal_inputs_${label}`].textInput));

      // Close the modal
      try {
        await page.press('Escape', { timeout: 1000 });
        await page.waitForTimeout(400);
      } catch (_) {}
    } catch (e) {
      console.log("  modal open failed:", e.message.slice(0, 80));
    }

    // ── /budgets ──────────────────────────────────────────────────────────
    console.log(`\n  -- /budgets (${label}) --`);
    await nav(page, "/budgets");
    await page.waitForTimeout(800);

    const budgetsShot = `gx03_budgets_${label}.png`;
    await shot(page, budgetsShot);
    report.screenshots.push(budgetsShot);

    report.measurements[`budgets_badges_${label}`] = await measureBadgesAndPills(page);
    report.measurements[`budgets_progress_${label}`] = await measureProgressBars(page);
    report.measurements[`budgets_buttons_${label}`] = await measureButtons(page);
    console.log("  pill:", JSON.stringify(report.measurements[`budgets_badges_${label}`].pill));
    console.log("  bar:", JSON.stringify(report.measurements[`budgets_progress_${label}`].bar));

    // ── /goals ────────────────────────────────────────────────────────────
    console.log(`\n  -- /goals (${label}) --`);
    await nav(page, "/goals");
    await page.waitForTimeout(800);

    const goalsShot = `gx03_goals_${label}.png`;
    await shot(page, goalsShot);
    report.screenshots.push(goalsShot);

    report.measurements[`goals_badges_${label}`] = await measureBadgesAndPills(page);
    report.measurements[`goals_progress_${label}`] = await measureProgressBars(page);
    console.log("  pace-badge:", JSON.stringify(report.measurements[`goals_badges_${label}`].paceBadge));
    console.log("  progress:", JSON.stringify(report.measurements[`goals_progress_${label}`].bar));

    // ── /todo — for priority badge variants ───────────────────────────────
    console.log(`\n  -- /todo (${label}) --`);
    await nav(page, "/todo");
    await page.waitForTimeout(800);

    const todoShot = `gx03_todo_${label}.png`;
    await shot(page, todoShot);
    report.screenshots.push(todoShot);

    report.measurements[`todo_badges_${label}`] = await measureBadgesAndPills(page);
    console.log("  prio-high:", JSON.stringify(report.measurements[`todo_badges_${label}`].prioHigh));

    // ── /accounts — for stat grid, buttons, misc ──────────────────────────
    if (width === 1280) {
      console.log(`\n  -- /accounts (${label}) --`);
      await nav(page, "/accounts");
      await page.waitForTimeout(800);
      const accShot = `gx03_accounts_${label}.png`;
      await shot(page, accShot);
      report.screenshots.push(accShot);
      report.measurements[`accounts_buttons_${label}`] = await measureButtons(page);
      report.measurements[`accounts_tooltips_${label}`] = await measureTooltips(page);
    }

    if (errors.length > 0) {
      console.log("  page errors:", errors.slice(0, 3));
    }
    await ctx.close();
  }
}

// ── Save measurements JSON ─────────────────────────────────────────────────
const meaPath = path.join(SHOTS_DIR, "gx03_measurements.json");
fs.writeFileSync(meaPath, JSON.stringify(report, null, 2));
console.log(`\nMeasurements saved: ${meaPath}`);
console.log(`Total screenshots: ${report.screenshots.length}`);
console.log("\nFull measurements:");
console.log(JSON.stringify(report.measurements, null, 2));

await browser.close();
process.exit(0);
