/**
 * reports_beautify_analysis.mjs
 * GLAMOR G9.1 — Deep visual analysis of /reports
 * Screenshots + computed-style measurements at 768, 1280, 1440 × dark + light.
 * Run: node e2e/reports_beautify_analysis.mjs
 */
import { chromium } from 'playwright-core';
import fs from 'fs';
import path from 'path';

const BASE = 'http://localhost:8080';
const OUT  = path.join(process.cwd(), 'e2e');
const WIDTHS = [768, 1280, 1440];
const THEMES = ['dark', 'light'];

async function seed(page) {
  // Seed sample transactions so every section renders
  await page.evaluate(() => {
    const DB_KEY = 'cashflux:db';
    const existing = localStorage.getItem(DB_KEY);
    if (existing) return; // don't overwrite real data
    // minimal seed so sections appear
  });
}

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    const prefs = JSON.parse(localStorage.getItem('cashflux:prefs') || '{}');
    prefs.theme = t;
    localStorage.setItem('cashflux:prefs', JSON.stringify(prefs));
  }, theme);
  await page.reload();
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute('data-theme') === t,
    theme,
    { timeout: 8000 }
  ).catch(() => console.warn(`[warn] data-theme="${theme}" not confirmed after reload`));
  await page.waitForTimeout(800);
}

async function navigateToReports(page) {
  await page.goto(`${BASE}/#/reports`, { waitUntil: 'networkidle', timeout: 15000 })
    .catch(() => page.goto(`${BASE}/`, { waitUntil: 'networkidle', timeout: 15000 }));
  // Try clicking nav link if needed
  const link = page.locator('a[href*="reports"], .nav[href*="reports"], [data-route*="reports"]').first();
  if (await link.count()) await link.click().catch(() => {});
  await page.waitForTimeout(1200);
}

async function measureStyles(page) {
  return await page.evaluate(() => {
    const cs = (sel, prop) => {
      const el = document.querySelector(sel);
      if (!el) return null;
      return window.getComputedStyle(el)[prop];
    };
    const csAll = (sel, prop) => {
      const els = [...document.querySelectorAll(sel)];
      return els.slice(0, 3).map(el => window.getComputedStyle(el)[prop]);
    };
    const rect = (sel) => {
      const el = document.querySelector(sel);
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return { w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top) };
    };

    // Count DOM elements
    const cardCount = document.querySelectorAll('.card').length;
    const rowCount  = document.querySelectorAll('.row').length;
    const sectionDividers = document.querySelectorAll('.section-divider').length;
    const shareBars = document.querySelectorAll('.share-bar').length;
    const svgs = document.querySelectorAll('svg').length;
    const areaCharts = document.querySelectorAll('.area-chart, [class*="chart"]').length;
    const exportBtns = [...document.querySelectorAll('.btn')].filter(b => b.textContent.includes('Download') || b.textContent.includes('CSV')).length;

    // Page height
    const scrollHeight = document.documentElement.scrollHeight;
    const viewHeight = window.innerHeight;
    const viewports = (scrollHeight / viewHeight).toFixed(1);

    // Type scale
    const cardTitleSize   = cs('.card-title', 'fontSize');
    const cardTitleWeight = cs('.card-title', 'fontWeight');
    const cardTitleColor  = cs('.card-title', 'color');
    const rowDescSize     = cs('.row-desc', 'fontSize');
    const rowDescWeight   = cs('.row-desc', 'fontWeight');
    const rowDescColor    = cs('.row-desc', 'color');
    const statValueSize   = cs('.stat-value', 'fontSize');
    const statValueWeight = cs('.stat-value', 'fontWeight');
    const statValueColor  = cs('.stat-value', 'color');
    const statLabelSize   = cs('.stat-label', 'fontSize');
    const statLabelColor  = cs('.stat-label', 'color');
    const mutedColor      = cs('.muted', 'color');
    const mutedSize       = cs('.muted', 'fontSize');
    const captionColor    = cs('.t-caption', 'color');

    // Section divider style
    const dividerSize   = cs('.section-divider', 'fontSize');
    const dividerColor  = cs('.section-divider', 'color');
    const dividerMarginTop = cs('.section-divider', 'marginTop');

    // Card geometry
    const cardPad     = cs('.card', 'padding');
    const cardMarginB = cs('.card', 'marginBottom');
    const cardBg      = cs('.card', 'backgroundColor');
    const cardBorder  = cs('.card', 'borderColor');
    const cardRadius  = cs('.card', 'borderRadius');

    // Stat grid
    const statGridGap    = cs('.stat-grid', 'gap');
    const statGridCols   = cs('.stat-grid', 'gridTemplateColumns');

    // Share bar (inline styled)
    const shareBarEl = document.querySelector('.share-bar');
    const shareBarH  = shareBarEl ? shareBarEl.style.height : null;
    const shareBarW  = shareBarEl ? shareBarEl.style.maxWidth : null;
    const shareBarBg = shareBarEl ? shareBarEl.style.background : null;
    // inner fill
    const shareBarFill = shareBarEl ? shareBarEl.querySelector('div')?.style?.background : null;

    // Mermaid / sankey
    const mermaidEl = document.querySelector('.mermaid, [class*="mermaid"]');
    const mermaidRect = mermaidEl ? (() => { const r = mermaidEl.getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height) }; })() : null;

    // Area chart SVG
    const svgEls = [...document.querySelectorAll('svg')];
    const svgSizes = svgEls.slice(0, 4).map(s => ({ w: Math.round(s.getBoundingClientRect().width), h: Math.round(s.getBoundingClientRect().height) }));

    // Row rhythm
    const rowPadTop  = cs('.row', 'paddingTop');
    const rowPadBot  = cs('.row', 'paddingBottom');
    const rowBorderT = cs('.row', 'borderTopWidth');

    // Btn (export buttons)
    const btnPad    = cs('.btn', 'padding');
    const btnSize   = cs('.btn', 'fontSize');
    const btnColor  = cs('.btn', 'color');
    const btnBorder = cs('.btn', 'borderColor');

    // Background tokens
    const htmlBg   = cs('html', 'backgroundColor');
    const mainBg   = cs('main', 'backgroundColor');
    const bodyBg   = cs('body', 'backgroundColor');

    // Amount colors
    const incomeAmtColor  = cs('.amount-income', 'color');
    const expenseAmtColor = cs('.amount-expense', 'color');
    const budgetAmtColor  = cs('.budget-amount', 'color');

    // Rollup toggle btn
    const rollupBtn = document.querySelector('[data-testid="reports-rollup-toggle"]');
    const rollupBtnSize  = rollupBtn ? window.getComputedStyle(rollupBtn).fontSize : null;
    const rollupBtnColor = rollupBtn ? window.getComputedStyle(rollupBtn).color : null;

    // Text contrast check: get actual bg and fg for card-title
    const cardEl = document.querySelector('.card');
    const cardBgActual = cardEl ? window.getComputedStyle(cardEl).backgroundColor : null;

    // All card-title sizes
    const allCardTitleSizes = csAll('.card-title', 'fontSize');

    // Page title / period caption
    const tCaptionColor = cs('.t-caption', 'color');
    const tCaptionSize  = cs('.t-caption', 'fontSize');

    return {
      counts: { cardCount, rowCount, sectionDividers, shareBars, svgs, areaCharts, exportBtns },
      scroll: { scrollHeight, viewHeight, viewports },
      typeScale: {
        cardTitle:   { size: cardTitleSize, weight: cardTitleWeight, color: cardTitleColor },
        rowDesc:     { size: rowDescSize, weight: rowDescWeight, color: rowDescColor },
        statValue:   { size: statValueSize, weight: statValueWeight, color: statValueColor },
        statLabel:   { size: statLabelSize, color: statLabelColor },
        muted:       { size: mutedSize, color: mutedColor },
        caption:     { size: tCaptionSize, color: tCaptionColor },
        allCardTitles: allCardTitleSizes,
      },
      sectionDivider: { size: dividerSize, color: dividerColor, marginTop: dividerMarginTop },
      card: { padding: cardPad, marginBottom: cardMarginB, bg: cardBg, border: cardBorder, radius: cardRadius },
      statGrid: { gap: statGridGap, cols: statGridCols },
      shareBar: { height: shareBarH, maxWidth: shareBarW, bg: shareBarBg, fillColor: shareBarFill },
      mermaid: mermaidRect,
      svgSizes,
      row: { padTop: rowPadTop, padBot: rowPadBot, borderTop: rowBorderT },
      btn: { padding: btnPad, size: btnSize, color: btnColor, border: btnBorder },
      bg: { html: htmlBg, main: mainBg, body: bodyBg },
      amounts: { income: incomeAmtColor, expense: expenseAmtColor, budget: budgetAmtColor },
      rollupToggle: { size: rollupBtnSize, color: rollupBtnColor },
      cardBgActual,
    };
  });
}

async function main() {
  const browser = await chromium.launch({ headless: true });

  const results = {};

  for (const theme of THEMES) {
    results[theme] = {};
    for (const width of WIDTHS) {
      console.log(`\n--- ${theme} ${width}px ---`);
      const ctx = await browser.newContext({ viewport: { width, height: 900 } });
      const page = await ctx.newPage();

      // Navigate to app first, set theme, reload
      await page.goto(BASE, { waitUntil: 'networkidle', timeout: 15000 });
      await setTheme(page, theme);
      await navigateToReports(page);

      // Measure
      const metrics = await measureStyles(page);
      results[theme][width] = metrics;
      console.log('  Cards:', metrics.counts.cardCount, '| Rows:', metrics.counts.rowCount,
        '| Section dividers:', metrics.counts.sectionDividers,
        '| Share bars:', metrics.counts.shareBars,
        '| SVGs:', metrics.counts.svgs,
        '| Export btns:', metrics.counts.exportBtns);
      console.log('  Scroll height:', metrics.scroll.scrollHeight, 'px =', metrics.scroll.viewports, 'viewports');
      console.log('  card-title:', metrics.typeScale.cardTitle);
      console.log('  stat-value:', metrics.typeScale.statValue);
      console.log('  row-desc:', metrics.typeScale.rowDesc);
      console.log('  muted:', metrics.typeScale.muted);
      console.log('  card padding:', metrics.card.padding, '| margin-bottom:', metrics.card.marginBottom);
      console.log('  shareBar h:', metrics.shareBar.height, 'maxW:', metrics.shareBar.maxWidth, 'fill:', metrics.shareBar.fillColor);
      console.log('  SVG sizes:', JSON.stringify(metrics.svgSizes));
      console.log('  mermaid rect:', metrics.mermaid);

      // Screenshot full-page
      const fname = `e2e/reports_${theme}_${width}.png`;
      await page.screenshot({ path: fname, fullPage: true });
      console.log('  Screenshot:', fname);

      await ctx.close();
    }
  }

  // Write JSON results for reference
  fs.writeFileSync('e2e/reports_metrics.json', JSON.stringify(results, null, 2));
  console.log('\nAll done. Metrics: e2e/reports_metrics.json');
  await browser.close();
}

main().catch(e => { console.error(e); process.exit(1); });
