/**
 * GX13 — Budgets page re-check (post G4 contrast fixes)
 * Run: node e2e/gx_13_budgets_recheck.mjs
 *
 * PART A: Regression confirm — measure actual computed colors on real elements
 * PART B: Residual blemishes — severity sort, near-limit pill, separators, bar tracks, sub-line
 *
 * Navigation strategy: Click the "Budgets" nav link from within the running app
 * (stale WASM doesn't honor /#budgets on cold load).
 */

import { chromium } from 'playwright';
import { mkdirSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const SCREENSHOTS = join(__dirname, 'screenshots');
mkdirSync(SCREENSHOTS, { recursive: true });

const BASE_URL = 'http://127.0.0.1:8080';
const WIDTHS = [1280, 1440, 768];
const THEMES = ['light', 'dark'];

// Navigate to budgets by clicking the nav link (handles stale WASM hash routing)
async function navigateToBudgets(page) {
  // Try clicking the nav link with aria-label or text "Budgets"
  // The nav uses class="nv" with text content
  try {
    // Wait for WASM to boot — look for the navigation rail
    await page.waitForSelector('.rail, aside, nav', { timeout: 8000 });
    // Click the Budgets nav item
    const navBudgets = page.locator('a[href*="budgets"], button:has-text("Budgets"), .nv:has-text("Budgets"), [data-testid*="budgets"]').first();
    const count = await navBudgets.count();
    if (count > 0) {
      await navBudgets.click();
      await page.waitForTimeout(1500);
      return { method: 'nav-click', url: page.url() };
    }
    // Fallback: set hash directly and wait longer
    await page.evaluate(() => { window.location.hash = '#budgets'; });
    await page.waitForTimeout(2000);
    return { method: 'hash', url: page.url() };
  } catch (e) {
    return { method: 'fallback', error: e.message, url: page.url() };
  }
}

// Get computed style for a CSS selector — returns first matching element
async function computedFirst(page, selector) {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const style = window.getComputedStyle(el);
    return {
      color: style.color,
      backgroundColor: style.backgroundColor,
      borderBottomColor: style.borderBottomColor,
      borderBottomWidth: style.borderBottomWidth,
      fontWeight: style.fontWeight,
      fontSize: style.fontSize,
      selector: sel,
      text: el.textContent?.trim().slice(0, 80),
      tagName: el.tagName,
      classList: Array.from(el.classList).join(' '),
    };
  }, selector);
}

// Relative luminance + contrast ratio helpers
function luminance(r, g, b) {
  return [r, g, b].reduce((sum, c, i) => {
    c /= 255;
    return sum + (i === 0 ? 0.2126 : i === 1 ? 0.7152 : 0.0722) *
      (c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4));
  }, 0);
}
function contrast(fg, bg) {
  const parse = s => { const m = s?.match(/(\d+(?:\.\d+)?),\s*(\d+(?:\.\d+)?),\s*(\d+(?:\.\d+)?)/); return m ? [+m[1],+m[2],+m[3]] : null; };
  const fc = parse(fg), bc = parse(bg);
  if (!fc || !bc) return null;
  const [L1, L2] = [luminance(...fc), luminance(...bc)];
  return +((Math.max(L1,L2)+0.05)/(Math.min(L1,L2)+0.05)).toFixed(2);
}

async function runProbe() {
  const browser = await chromium.launch({ headless: true });
  const report = {
    meta: { date: '2026-06-22', url: BASE_URL, widths: WIDTHS, themes: THEMES, exitCode: 0 },
    results: {}
  };

  for (const theme of THEMES) {
    report.results[theme] = {};

    for (const width of WIDTHS) {
      const page = await browser.newPage();
      await page.setViewportSize({ width, height: 900 });

      // Set theme in localStorage BEFORE first navigation
      await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 15000 });
      await page.evaluate((t) => {
        localStorage.setItem('theme', t);
        localStorage.setItem('cashflux-theme', t);
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      // Wait for WASM to fully boot (look for the nav or app content)
      await page.waitForTimeout(3000);

      // Navigate to Budgets
      const navResult = await navigateToBudgets(page);

      // Force theme attr again after navigation (WASM may reset it)
      await page.evaluate((t) => {
        localStorage.setItem('theme', t);
        document.documentElement.setAttribute('data-theme', t);
      }, theme);
      await page.waitForTimeout(1000);

      const screenshotPath = join(SCREENSHOTS, `gx13_budgets_${theme}_${width}.png`);
      await page.screenshot({ path: screenshotPath, fullPage: true });

      // Dump the DOM structure on the budgets page for diagnostics
      const domDump = await page.evaluate(() => {
        const budgetEls = document.querySelectorAll('[class*="budget"]');
        const budgetInfo = Array.from(budgetEls).slice(0, 8).map(el => ({
          tag: el.tagName,
          classes: Array.from(el.classList).join(' '),
          text: el.textContent?.trim().slice(0, 60),
          childCount: el.children.length,
        }));
        const statGridEls = document.querySelectorAll('.stat-grid, .stat, .stat-value, .stat-label');
        const statInfo = Array.from(statGridEls).slice(0, 6).map(el => ({
          tag: el.tagName,
          classes: Array.from(el.classList).join(' '),
          text: el.textContent?.trim().slice(0, 40),
        }));
        return {
          budgetEls: budgetInfo,
          statEls: statInfo,
          route: location.hash || location.pathname,
          title: document.title,
          rowCount: document.querySelectorAll('.row').length,
          budgetDivCount: document.querySelectorAll('.budget').length,
          budgetHeadCount: document.querySelectorAll('.budget-head').length,
          barCount: document.querySelectorAll('.bar').length,
          barFillCount: document.querySelectorAll('.bar-fill').length,
          budgetSubCount: document.querySelectorAll('.budget-sub').length,
          budgetAmountCount: document.querySelectorAll('.budget-amount').length,
          rowDescCount: document.querySelectorAll('.row-desc').length,
          statGridCount: document.querySelectorAll('.stat-grid').length,
          statValueCount: document.querySelectorAll('.stat-value').length,
          themeAttr: document.documentElement.getAttribute('data-theme'),
        };
      });

      const result = { nav: navResult, width, theme, screenshotPath, domDump };

      // ── PART A: Regression confirm (G4 light-mode contrast fixes) ──────────────
      result.partA = {};

      // A1. Category name — .budget-head .row-desc (the actual name in a budget row)
      const rowDescInBudget = await computedFirst(page, '.budget-head .row-desc');
      const rowDescDrillable = await computedFirst(page, '.budget-head .budget-drill');
      result.partA.categoryName = rowDescInBudget || rowDescDrillable;

      // A2. Stat figures — BUDGETED/SPENT/LEFT (.stat-value)
      const statValue = await computedFirst(page, '.stat-grid .stat-value');
      result.partA.statValue = statValue;

      // A2b. Budget amount (spent / limit) in budget-head
      const budgetAmount = await computedFirst(page, '.budget-head .budget-amount');
      result.partA.budgetAmount = budgetAmount;

      // A3. .budget-amount (all)
      const budgetAmountFirst = await computedFirst(page, '.budget-amount');
      result.partA.budgetAmountFirst = budgetAmountFirst;

      // A4. Progress bar fill
      const barFill = await page.evaluate(() => {
        const fills = document.querySelectorAll('.bar-fill');
        if (!fills.length) return { found: false, count: 0 };
        return Array.from(fills).slice(0, 5).map(el => {
          const style = window.getComputedStyle(el);
          return {
            classList: Array.from(el.classList).join(' '),
            width: el.style.width || style.width,
            background: style.backgroundColor,
          };
        });
      });
      result.partA.progressBarFills = barFill;

      // A4b. Bar track
      const barTrack = await page.evaluate(() => {
        const tracks = document.querySelectorAll('.bar');
        if (!tracks.length) return { found: false };
        const el = tracks[0];
        const style = window.getComputedStyle(el);
        return {
          found: true,
          background: style.backgroundColor,
          height: style.height,
          classList: Array.from(el.classList).join(' '),
        };
      });
      result.partA.barTrack = barTrack;

      // A5. Sub-line text (.budget-sub)
      const subLine = await computedFirst(page, '.budget-sub');
      result.partA.subLine = subLine;

      // Compute contrast ratios against white card bg for light theme
      const whiteBg = 'rgb(255, 255, 255)';
      const cardBg = theme === 'light' ? whiteBg : null;

      if (cardBg) {
        const cn = result.partA.categoryName;
        if (cn) result.partA.categoryNameContrast = contrast(cn.color, cardBg);

        const sv = result.partA.statValue;
        if (sv) result.partA.statValueContrast = contrast(sv.color, cardBg);

        const ba = result.partA.budgetAmountFirst;
        if (ba) result.partA.budgetAmountContrast = contrast(ba.color, cardBg);

        const sl = result.partA.subLine;
        if (sl) result.partA.subLineContrast = contrast(sl.color, cardBg);
      }

      // ── PART B: Residual blemishes ─────────────────────────────────────────────
      result.partB = {};

      // B1. Row ORDER — budget rows + their state classes
      const rowOrder = await page.evaluate(() => {
        const budgets = document.querySelectorAll('.budget');
        return Array.from(budgets).slice(0, 15).map(el => {
          const nameEl = el.querySelector('.row-desc, .budget-drill');
          const barFill = el.querySelector('.bar-fill');
          const subLines = Array.from(el.querySelectorAll('.budget-sub')).map(s => s.textContent?.trim().slice(0, 60));
          return {
            name: nameEl?.textContent?.trim().slice(0, 50),
            barFillClass: barFill ? Array.from(barFill.classList).join(' ') : null,
            subLines,
          };
        });
      });
      result.partB.rowOrder = rowOrder;

      // B2. NEAR-LIMIT PILL — look for .pill.text-warn (amber) in the summary area
      const pills = await page.evaluate(() => {
        const pills = document.querySelectorAll('.pill');
        return Array.from(pills).map(el => {
          const style = window.getComputedStyle(el);
          return {
            text: el.textContent?.trim().slice(0, 40),
            classList: Array.from(el.classList).join(' '),
            color: style.color,
            bg: style.backgroundColor,
          };
        });
      });
      result.partB.pills = pills;

      // B3. ROW SEPARATORS — .budget divs: measure gap/border
      const rowSep = await page.evaluate(() => {
        const budgets = document.querySelectorAll('.budget');
        if (budgets.length < 2) return { found: false, count: budgets.length };
        const el = budgets[0];
        const style = window.getComputedStyle(el);
        return {
          found: true,
          count: budgets.length,
          borderBottom: style.borderBottom,
          borderBottomColor: style.borderBottomColor,
          borderBottomWidth: style.borderBottomWidth,
          marginBottom: style.marginBottom,
          paddingBottom: style.paddingBottom,
        };
      });
      result.partB.rowSeparator = rowSep;

      // B4. EMPTY BAR TRACKS — check track bg visibility for 0% fills
      const emptyBars = await page.evaluate(() => {
        const fills = document.querySelectorAll('.bar-fill');
        const zeroBars = Array.from(fills).filter(el => el.style.width === '0%' || el.style.width === '0');
        const tracks = document.querySelectorAll('.bar');
        const firstTrack = tracks[0];
        const trackStyle = firstTrack ? window.getComputedStyle(firstTrack) : null;
        return {
          totalFills: fills.length,
          zeroWidthFills: zeroBars.length,
          trackCount: tracks.length,
          firstTrackBg: trackStyle?.backgroundColor,
          firstTrackHeight: trackStyle?.height,
        };
      });
      result.partB.emptyBars = emptyBars;

      // B5. SUB-LINE HIERARCHY — are the sub-lines multiple distinct <span>s or a run-on?
      const subLineStructure = await page.evaluate(() => {
        const budgets = document.querySelectorAll('.budget');
        return Array.from(budgets).slice(0, 3).map(el => {
          const subs = Array.from(el.querySelectorAll('.budget-sub'));
          return {
            name: el.querySelector('.row-desc, .budget-drill')?.textContent?.trim().slice(0, 40),
            subLineCount: subs.length,
            subLines: subs.map(s => ({
              text: s.textContent?.trim().slice(0, 80),
              classList: Array.from(s.classList).join(' '),
              color: window.getComputedStyle(s).color,
            })),
          };
        });
      });
      result.partB.subLineStructure = subLineStructure;

      // B6. PROGRESS BAR COLORS — semantic color check
      const barColors = await page.evaluate(() => {
        const fills = document.querySelectorAll('.bar-fill');
        return Array.from(fills).slice(0, 10).map(el => {
          const style = window.getComputedStyle(el);
          return {
            classList: Array.from(el.classList).join(' '),
            background: style.backgroundColor,
            width: el.style.width,
          };
        });
      });
      result.partB.barColors = barColors;

      // B7. STAT GRID — measure background for the stat card/grid
      const statGrid = await page.evaluate(() => {
        const grid = document.querySelector('.stat-grid');
        if (!grid) return null;
        const style = window.getComputedStyle(grid);
        const stats = Array.from(grid.querySelectorAll('.stat')).map(el => {
          const label = el.querySelector('.stat-label');
          const value = el.querySelector('.stat-value');
          const labelStyle = label ? window.getComputedStyle(label) : null;
          const valueStyle = value ? window.getComputedStyle(value) : null;
          return {
            labelText: label?.textContent?.trim(),
            valueText: value?.textContent?.trim(),
            labelColor: labelStyle?.color,
            valueColor: valueStyle?.color,
          };
        });
        return {
          background: style.backgroundColor,
          stats,
        };
      });
      result.partB.statGrid = statGrid;

      report.results[theme][width] = result;
      await page.close();
    }
  }

  await browser.close();
  console.log(JSON.stringify(report, null, 2));
}

runProbe().catch(err => {
  console.error('PROBE ERROR:', err);
  process.exit(1);
});
