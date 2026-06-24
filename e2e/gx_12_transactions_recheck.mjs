/**
 * GX12 — Transactions re-check (post-fixes)
 * "The Reconciler, Revisited"
 *
 * Regression-confirms GX3-F1/F2 fixes on the Transactions page and probes
 * residual structure blemishes. Runs light + dark at 1280, 1440, 768.
 */

import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const BASE = 'http://localhost:8099';
const SCREENSHOTS_DIR = path.resolve('e2e/screenshots');
fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });

const WIDTHS = [1280, 1440, 768];
const HEIGHT = 900;

/** Helper: navigate to /transactions via SPA push (avoids WASM reload timeout) */
async function navToTransactions(page) {
  await page.evaluate(() => {
    window.history.pushState({}, '', '/transactions');
    window.dispatchEvent(new PopStateEvent('popstate', { state: {} }));
  });
  // Wait for table OR empty state
  await page.waitForFunction(() => {
    return document.querySelector('.txn-table') || document.querySelector('.empty') || document.querySelector('[data-page="transactions"]');
  }, { timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(600);
}

async function setTheme(page, theme) {
  await page.evaluate((t) => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t })), theme);
  await page.reload();
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute('data-theme') === t,
    theme,
    { timeout: 8000 }
  );
  await page.waitForTimeout(500);
}

async function getStyle(page, selector, prop) {
  try {
    await page.waitForSelector(selector, { timeout: 5000 });
    return await page.evaluate(([sel, p]) => {
      const el = document.querySelector(sel);
      if (!el) return 'N/A (not found)';
      return window.getComputedStyle(el).getPropertyValue(p);
    }, [selector, prop]);
  } catch {
    return 'N/A (selector timeout)';
  }
}

async function getStyles(page, selector, props) {
  try {
    await page.waitForSelector(selector, { timeout: 5000 });
    return await page.evaluate(([sel, ps]) => {
      const el = document.querySelector(sel);
      if (!el) return Object.fromEntries(ps.map(p => [p, 'N/A (not found)']));
      const cs = window.getComputedStyle(el);
      return Object.fromEntries(ps.map(p => [p, cs.getPropertyValue(p)]));
    }, [selector, props]);
  } catch {
    return Object.fromEntries(props.map(p => [p, 'N/A (selector timeout)']));
  }
}

async function clientHeight(page, selector) {
  try {
    await page.waitForSelector(selector, { timeout: 5000 });
    return await page.evaluate((sel) => {
      const el = document.querySelector(sel);
      return el ? el.clientHeight : -1;
    }, selector);
  } catch {
    return -1;
  }
}

// ── MAIN ──────────────────────────────────────────────────────────────────────

const browser = await chromium.launch({ headless: true });
const results = { partA: {}, partB: {} };

// We'll collect structured results per theme+width
const allMeasurements = [];

for (const theme of ['light', 'dark']) {
  for (const width of WIDTHS) {
    const ctx = await browser.newContext({ viewport: { width, height: HEIGHT } });
    const page = await ctx.newPage();

    // Boot: load root so WASM caches, then set theme
    await page.goto(BASE + '/', { waitUntil: 'networkidle', timeout: 30000 });
    await setTheme(page, theme);
    await navToTransactions(page);

    const screenshotName = `gx12_${theme}_${width}.png`;
    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, screenshotName), fullPage: false });

    const m = { theme, width, screenshot: screenshotName };

    // ── PART A: Regression confirms ───────────────────────────────────────────

    // A1: thead th background (GX3-F2 fix)
    m.theadBg = await getStyle(page, '.txn-table thead th', 'background-color');

    // A2: tbody td color (GX3-F2 fix)
    m.tbodyColor = await getStyle(page, '.txn-table tbody td', 'color');

    // A3: filter bar select height (GX3-F1 fix)
    try {
      await page.waitForSelector('.filter-bar select, .txn-filters select, select', { timeout: 5000 });
      m.selectHeight = await page.evaluate(() => {
        const sel = document.querySelector('.filter-bar select') ||
                    document.querySelector('.txn-filters select') ||
                    document.querySelector('select');
        return sel ? sel.getBoundingClientRect().height : -1;
      });
      m.selectBg = await page.evaluate(() => {
        const sel = document.querySelector('.filter-bar select') ||
                    document.querySelector('.txn-filters select') ||
                    document.querySelector('select');
        return sel ? window.getComputedStyle(sel).getPropertyValue('background-color') : 'N/A';
      });
    } catch {
      m.selectHeight = -1;
      m.selectBg = 'N/A';
    }

    // A4: Zebra striping — even vs odd differ?
    m.zebraEven = await getStyle(page, '.txn-table tbody tr.row:nth-child(even)', 'background-color');
    m.zebraOdd = await getStyle(page, '.txn-table tbody tr.row:nth-child(odd)', 'background-color');

    // A5: Row hover — programmatically hover a row
    m.rowHoverBg = 'N/A (no rows)';
    try {
      const hasRow = await page.evaluate(() => !!document.querySelector('.txn-table tbody tr.row'));
      if (hasRow) {
        await page.hover('.txn-table tbody tr.row');
        await page.waitForTimeout(150);
        m.rowHoverBg = await page.evaluate(() => {
          const el = document.querySelector('.txn-table tbody tr.row');
          return el ? window.getComputedStyle(el).getPropertyValue('background-color') : 'N/A';
        });
      }
    } catch { /* leave N/A */ }

    // ── PART B: Residual blemishes ────────────────────────────────────────────

    // B1: Row height
    m.rowHeight = await clientHeight(page, '.txn-table tbody tr.row');

    // B2: Actions column width (last th)
    m.actionsColWidth = await page.evaluate(() => {
      const ths = Array.from(document.querySelectorAll('.txn-table thead th'));
      const actTh = ths.find(th => th.classList.contains('td-actions') ||
                              (th.textContent || '').trim() === '' ||
                              th.querySelector('.td-actions'));
      const lastTh = ths[ths.length - 1];
      const target = actTh || lastTh;
      return target ? target.getBoundingClientRect().width : -1;
    }).catch(() => -1);

    // B3: Amount text-align + font-variant-numeric
    m.amountTextAlign = await getStyle(page, '.txn-table .td-amount', 'text-align');
    m.amountFontVariant = await getStyle(page, '.txn-table .td-amount', 'font-variant-numeric');

    // B4: Description truncation
    const descStyles = await getStyles(page, '.txn-table td.row-desc', ['overflow', 'text-overflow', 'white-space']);
    m.descOverflow = descStyles['overflow'];
    m.descTextOverflow = descStyles['text-overflow'];
    m.descWhiteSpace = descStyles['white-space'];

    // B5: Header font-weight
    m.headerFontWeight = await getStyle(page, '.txn-table thead th', 'font-weight');

    // B6: Sort caret presence
    m.sortCaretPresent = await page.evaluate(() => {
      // Check for aria-sort on any th
      const sortedTh = document.querySelector('.txn-table thead th[aria-sort]');
      // Check for .th-sort elements (the sort button)
      const thSort = document.querySelector('.txn-table .th-sort');
      // Check for text content including ▲ or ▼
      const headers = Array.from(document.querySelectorAll('.txn-table thead th'));
      const hasCaretText = headers.some(h => h.textContent.includes('▲') || h.textContent.includes('▾') || h.textContent.includes('▼'));
      return {
        hasAriaSortAttr: !!sortedTh,
        hasThSortBtn: !!thSort,
        hasCaretText,
        thSortCount: document.querySelectorAll('.txn-table .th-sort').length,
      };
    }).catch(() => ({ hasAriaSortAttr: false, hasThSortBtn: false, hasCaretText: false, thSortCount: 0 }));

    // B7: Selected row styling
    m.selectedRowBg = await getStyle(page, '.txn-table tbody tr.selected', 'background-color');

    // B8: Checkbox column width (first th)
    m.checkboxColWidth = await page.evaluate(() => {
      const firstTh = document.querySelector('.txn-table thead th:first-child');
      return firstTh ? firstTh.getBoundingClientRect().width : -1;
    }).catch(() => -1);

    // B9: Filter bar alignment — does the filter bar exist + what flex direction?
    m.filterBarDisplay = await page.evaluate(() => {
      const fb = document.querySelector('.filter-bar') || document.querySelector('.txn-filters');
      if (!fb) return 'N/A (not found)';
      const cs = window.getComputedStyle(fb);
      return cs.getPropertyValue('display') + ' / flex-dir: ' + cs.getPropertyValue('flex-direction');
    }).catch(() => 'N/A');

    // B10: Table present at all?
    m.tablePresent = await page.evaluate(() => !!document.querySelector('.txn-table')).catch(() => false);

    // B11: Count of tbody rows
    m.rowCount = await page.evaluate(() => document.querySelectorAll('.txn-table tbody tr.row').length).catch(() => 0);

    // B12: Column header count
    m.headerCount = await page.evaluate(() => document.querySelectorAll('.txn-table thead th').length).catch(() => 0);

    allMeasurements.push(m);
    await ctx.close();
  }
}

await browser.close();

// ── REPORT ────────────────────────────────────────────────────────────────────

console.log('\n═══════════════════════════════════════════════════════════');
console.log('  GX12 — Transactions Re-check Report');
console.log('═══════════════════════════════════════════════════════════\n');

// PART A — Regression confirms (focus on light mode 1280 as the canonical probe)
const lightRef = allMeasurements.find(m => m.theme === 'light' && m.width === 1280);
const darkRef  = allMeasurements.find(m => m.theme === 'dark'  && m.width === 1280);

console.log('PART A — Regression Confirms (GX3-F1/F2 fixes)\n');

function passIf(cond, desc, measured, expected) {
  const result = cond ? '✅ PASS' : '❌ FAIL';
  console.log(`  ${result}  ${desc}`);
  console.log(`          Expected: ${expected}`);
  console.log(`          Measured: ${measured}`);
}

if (lightRef) {
  // A1: thead bg in light mode should be warm white ~#f7f6f3 = rgb(247,246,243)
  const theadBgOk = lightRef.theadBg.includes('247') && lightRef.theadBg.includes('246');
  passIf(theadBgOk,
    'A1 [LIGHT] .txn-table thead th background-color',
    lightRef.theadBg,
    'rgb(247,246,243) — warm white (GX3-F2 fix)'
  );

  // A2: tbody td color in light mode should be dark text
  const tbodyColorOk = (() => {
    const v = lightRef.tbodyColor;
    // rgb(28,28,30) or similar dark value; reject near-white rgb(244...) or rgb(255...)
    const match = v.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
    if (!match) return false;
    const lum = (parseInt(match[1]) + parseInt(match[2]) + parseInt(match[3])) / 3;
    return lum < 100; // dark text
  })();
  passIf(tbodyColorOk,
    'A2 [LIGHT] .txn-table tbody td color (should be dark text)',
    lightRef.tbodyColor,
    'dark text ~rgb(28,28,30) — GX3-F2 fix'
  );

  // A3: select height >= 40px
  const selectOk = lightRef.selectHeight >= 40;
  passIf(selectOk,
    `A3 [LIGHT] filter bar select height (≥40px)`,
    `${lightRef.selectHeight}px`,
    '≥ 40px (GX3-F1 fix)'
  );

  console.log(`         select background: ${lightRef.selectBg}`);

  // A4: Zebra striping
  const zebrasDiffer = lightRef.zebraEven !== lightRef.zebraOdd &&
    lightRef.zebraEven !== 'N/A (selector timeout)' &&
    lightRef.zebraOdd  !== 'N/A (selector timeout)';
  passIf(zebrasDiffer,
    'A4 [LIGHT] Zebra striping: even ≠ odd row background',
    `even: ${lightRef.zebraEven}  odd: ${lightRef.zebraOdd}`,
    'even and odd should differ'
  );

  // A5: Row hover visible
  const hoverOk = lightRef.rowHoverBg !== 'N/A (no rows)' &&
    !lightRef.rowHoverBg.includes('N/A');
  passIf(hoverOk,
    'A5 [LIGHT] Row hover background-color (programmatic hover)',
    lightRef.rowHoverBg,
    '#efede8 or similar warm surface (GX3-F2 fix)'
  );
}

if (darkRef) {
  console.log('');
  // A1d: thead bg in dark mode should be dark
  const theadDarkOk = (() => {
    const v = darkRef.theadBg;
    const match = v.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
    if (!match) return false;
    const lum = (parseInt(match[1]) + parseInt(match[2]) + parseInt(match[3])) / 3;
    return lum < 60;
  })();
  passIf(theadDarkOk,
    'A1d [DARK] .txn-table thead th background-color (should remain dark)',
    darkRef.theadBg,
    'dark ~rgb(14,14,15) or similar'
  );

  // A3d: select height in dark
  const selectDarkOk = darkRef.selectHeight >= 40;
  passIf(selectDarkOk,
    `A3d [DARK] filter bar select height (≥40px)`,
    `${darkRef.selectHeight}px`,
    '≥ 40px (GX3-F1 fix)'
  );
}

console.log('\n─────────────────────────────────────────────────────────');
console.log('PART B — Residual Blemish Measurements (all combos)\n');

for (const m of allMeasurements) {
  console.log(`  [${m.theme.toUpperCase()} @ ${m.width}px]  screenshot: ${m.screenshot}`);
  console.log(`    Table present: ${m.tablePresent}  |  row count: ${m.rowCount}  |  header cols: ${m.headerCount}`);
  console.log(`    Row height (clientHeight): ${m.rowHeight}px`);
  console.log(`    Actions col width (last/actions th): ${m.actionsColWidth}px`);
  console.log(`    Checkbox col width (1st th): ${m.checkboxColWidth}px`);
  console.log(`    Amount text-align: ${m.amountTextAlign}  |  font-variant-numeric: ${m.amountFontVariant}`);
  console.log(`    Desc overflow: ${m.descOverflow}  |  text-overflow: ${m.descTextOverflow}  |  white-space: ${m.descWhiteSpace}`);
  console.log(`    Header font-weight: ${m.headerFontWeight}`);
  console.log(`    Sort carets: aria-sort=${m.sortCaretPresent?.hasAriaSortAttr}, th-sort-btns=${m.sortCaretPresent?.thSortCount}, caret-text=${m.sortCaretPresent?.hasCaretText}`);
  console.log(`    Selected row bg: ${m.selectedRowBg}`);
  console.log(`    Filter bar layout: ${m.filterBarDisplay}`);
  console.log(`    Zebra even: ${m.zebraEven}  |  odd: ${m.zebraOdd}`);
  console.log(`    Row hover bg: ${m.rowHoverBg}`);
  console.log('');
}

console.log('\nScreenshots saved to e2e/screenshots/:');
allMeasurements.forEach(m => console.log('  ' + m.screenshot));
console.log('');
