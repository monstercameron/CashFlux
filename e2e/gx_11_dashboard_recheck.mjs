/**
 * GX11 — Dashboard re-check (post-fixes)
 * Viewports: 1280, 1440, 768 × themes: light, dark
 * PART A: regression checks (GX1/GX2 landed fixes)
 * PART B: residual structure measurements
 */

import { chromium } from '../.tools/node_modules/playwright/index.mjs';
import { mkdir } from 'fs/promises';
import { existsSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOTS_DIR = path.join(__dirname, 'screenshots');
const BASE_URL = 'http://localhost:8080';

await mkdir(SCREENSHOTS_DIR, { recursive: true });

const VIEWPORTS = [1280, 1440, 768];
const THEMES = ['light', 'dark'];

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
  }, theme);
  await page.reload();
  if (theme === 'light') {
    await page.waitForFunction(
      () => document.documentElement.getAttribute('data-theme') === 'light',
      { timeout: 8000 }
    ).catch(() => console.warn('WARN: data-theme light not confirmed'));
  } else {
    await page.waitForFunction(
      () => document.documentElement.getAttribute('data-theme') === 'dark' ||
            !document.documentElement.hasAttribute('data-theme'),
      { timeout: 8000 }
    ).catch(() => console.warn('WARN: data-theme dark not confirmed'));
  }
  // Wait for WASM/app to settle
  await page.waitForTimeout(1500);
}

function rgb(str) { return str || 'N/A'; }

async function measureEl(page, sel, props) {
  try {
    return await page.evaluate(([s, ps]) => {
      const el = document.querySelector(s);
      if (!el) return { _missing: true };
      const cs = getComputedStyle(el);
      const r = {};
      for (const p of ps) r[p] = cs.getPropertyValue(p).trim();
      const rect = el.getBoundingClientRect();
      r._rect = { x: rect.x, y: rect.y, w: rect.width, h: rect.height };
      return r;
    }, [sel, props]);
  } catch (e) {
    return { _error: e.message };
  }
}

async function safeEval(page, fn, label) {
  try {
    return await page.evaluate(fn);
  } catch (e) {
    console.log(`  WARN [${label}]: ${e.message}`);
    return null;
  }
}

const browser = await chromium.launch({ headless: true });
const allResults = {};

for (const width of VIEWPORTS) {
  for (const theme of THEMES) {
    const key = `${width}_${theme}`;
    console.log(`\n========== ${width}px / ${theme} ==========`);

    const context = await browser.newContext({
      viewport: { width, height: 900 },
    });
    const page = await context.newPage();

    // Navigate to dashboard
    await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle', timeout: 15000 })
      .catch(() => page.goto(`${BASE_URL}/`, { timeout: 15000 }));

    await setTheme(page, theme);
    await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle', timeout: 10000 })
      .catch(() => {});
    await page.waitForTimeout(1000);

    // ---- PART A: Regression checks ----

    // A1: shell topbar/rail background (GX1 fix)
    let partA1 = {};
    try {
      partA1 = await page.evaluate(() => {
        const topbar = document.querySelector('.topbar') || document.querySelector('header');
        const rail = document.querySelector('aside.rail') || document.querySelector('.rail');
        return {
          topbarBg: topbar ? getComputedStyle(topbar).backgroundColor : '_missing',
          railBg: rail ? getComputedStyle(rail).backgroundColor : '_missing',
        };
      });
      console.log(`PART_A_1 topbar bg: ${partA1.topbarBg}`);
      console.log(`PART_A_1 rail bg: ${partA1.railBg}`);
    } catch (e) { console.log(`PART_A_1 ERROR: ${e.message}`); }

    // A2: card title + amount text color vs background
    let partA2 = {};
    try {
      partA2 = await page.evaluate(() => {
        const results = {};
        // Look for card title elements
        const titleSels = ['.wtitle', '.card-title', '.widget-title', '.whead h2', '.whead h3', '.whead span'];
        for (const s of titleSels) {
          const el = document.querySelector(s);
          if (el) {
            const cs = getComputedStyle(el);
            results.cardTitle = { sel: s, color: cs.color, bg: cs.backgroundColor };
            break;
          }
        }
        // Look for amount/value elements
        const amountSels = ['.amount', '.value', '.net-worth', '.wamount', '[class*="amount"]', '[class*="value"]'];
        for (const s of amountSels) {
          const el = document.querySelector(s);
          if (el) {
            const cs = getComputedStyle(el);
            results.amount = { sel: s, color: cs.color, bg: cs.backgroundColor };
            break;
          }
        }
        // Card background
        const cardSels = ['.card', '.widget', '.bento-item', '[class*="card"]'];
        for (const s of cardSels) {
          const el = document.querySelector(s);
          if (el) {
            const cs = getComputedStyle(el);
            results.cardBg = { sel: s, bg: cs.backgroundColor };
            break;
          }
        }
        return results;
      });
      console.log(`PART_A_2 card title: ${JSON.stringify(partA2.cardTitle)}`);
      console.log(`PART_A_2 amount: ${JSON.stringify(partA2.amount)}`);
      console.log(`PART_A_2 card bg: ${JSON.stringify(partA2.cardBg)}`);
    } catch (e) { console.log(`PART_A_2 ERROR: ${e.message}`); }

    // A3: inter-card/page background (no dark bleed in light mode)
    let partA3 = {};
    try {
      partA3 = await page.evaluate(() => {
        const body = document.body;
        const main = document.querySelector('main') || document.querySelector('.main') || document.querySelector('#app');
        return {
          bodyBg: getComputedStyle(body).backgroundColor,
          mainBg: main ? getComputedStyle(main).backgroundColor : '_missing',
          dataTheme: document.documentElement.getAttribute('data-theme'),
        };
      });
      console.log(`PART_A_3 body bg: ${partA3.bodyBg}`);
      console.log(`PART_A_3 main bg: ${partA3.mainBg}`);
      console.log(`PART_A_3 data-theme: ${partA3.dataTheme}`);
    } catch (e) { console.log(`PART_A_3 ERROR: ${e.message}`); }

    // A4: .empty text contrast (GX2-F5)
    let partA4 = {};
    try {
      partA4 = await page.evaluate(() => {
        const empties = document.querySelectorAll('.empty');
        const results = [];
        empties.forEach(el => {
          const cs = getComputedStyle(el);
          results.push({ color: cs.color, fontSize: cs.fontSize, bg: cs.backgroundColor });
        });
        return results.length ? results : [{ _missing: true }];
      });
      console.log(`PART_A_4 .empty text: ${JSON.stringify(partA4[0])}`);
    } catch (e) { console.log(`PART_A_4 ERROR: ${e.message}`); }

    // ---- PART B: Residual measurements ----

    // B1: net-worth tile dimensions vs other tiles
    let partB1 = {};
    try {
      partB1 = await page.evaluate(() => {
        // Try various selectors for net worth widget
        const nwSels = ['.net-worth', '[data-widget="net-worth"]', '.nw', '#net-worth',
          '[class*="net"]', '.bento-item:first-child', '.widget:first-child'];
        let nwEl = null;
        for (const s of nwSels) {
          const el = document.querySelector(s);
          if (el) { nwEl = el; break; }
        }
        const allItems = document.querySelectorAll('.bento-item, .widget, .card');
        const sizes = [];
        allItems.forEach((el, i) => {
          const r = el.getBoundingClientRect();
          if (r.width > 0 && r.height > 0 && i < 8) {
            sizes.push({ w: Math.round(r.width), h: Math.round(r.height) });
          }
        });
        return {
          nwRect: nwEl ? (() => { const r = nwEl.getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height), sel: 'found' }; })() : { _missing: true },
          tileCount: sizes.length,
          tileSizes: sizes.slice(0, 6),
        };
      });
      console.log(`PART_B_1 net-worth tile: ${JSON.stringify(partB1.nwRect)}`);
      console.log(`PART_B_1 all tiles (first 6): ${JSON.stringify(partB1.tileSizes)}`);
    } catch (e) { console.log(`PART_B_1 ERROR: ${e.message}`); }

    // B2: cash-flow surplus/deficit widget above fold?
    let partB2 = {};
    try {
      partB2 = await page.evaluate(() => {
        const sels = ['.cash-flow', '.surplus', '.deficit', '[class*="flow"]', '[class*="surplus"]',
          '[class*="cashflow"]', '.cf-widget'];
        for (const s of sels) {
          const el = document.querySelector(s);
          if (el) {
            const r = el.getBoundingClientRect();
            return { sel: s, y: Math.round(r.y), h: Math.round(r.height), aboveFold: r.y < window.innerHeight };
          }
        }
        // Fall back: check all widgets for text containing "surplus" or "cash flow"
        const allEls = document.querySelectorAll('.whead, .wtitle, [class*="title"]');
        for (const el of allEls) {
          const txt = el.textContent.toLowerCase();
          if (txt.includes('surplus') || txt.includes('deficit') || txt.includes('cash flow') || txt.includes('cashflow')) {
            const widget = el.closest('.bento-item, .widget, .card');
            if (widget) {
              const r = widget.getBoundingClientRect();
              return { sel: 'text-match:' + txt.trim().slice(0, 30), y: Math.round(r.y), h: Math.round(r.height), aboveFold: r.y < window.innerHeight };
            }
          }
        }
        return { _missing: true };
      });
      console.log(`PART_B_2 cash-flow widget: ${JSON.stringify(partB2)}`);
    } catch (e) { console.log(`PART_B_2 ERROR: ${e.message}`); }

    // B3: bento grid gap
    let partB3 = {};
    try {
      partB3 = await page.evaluate(() => {
        const bentoSels = ['.bento', '.bento-grid', '.dashboard-grid', '.widget-grid', 'main .grid'];
        for (const s of bentoSels) {
          const el = document.querySelector(s);
          if (el) {
            const cs = getComputedStyle(el);
            return {
              sel: s,
              gap: cs.gap,
              rowGap: cs.rowGap,
              columnGap: cs.columnGap,
              display: cs.display,
              gridTemplateColumns: cs.gridTemplateColumns,
            };
          }
        }
        // fallback: check main's direct grid children
        const main = document.querySelector('main');
        if (main) {
          const cs = getComputedStyle(main);
          if (cs.display.includes('grid') || cs.display.includes('flex')) {
            return { sel: 'main', gap: cs.gap, display: cs.display };
          }
        }
        return { _missing: true };
      });
      console.log(`PART_B_3 bento grid gap: ${JSON.stringify(partB3)}`);
    } catch (e) { console.log(`PART_B_3 ERROR: ${e.message}`); }

    // B4: widget padding consistency
    let partB4 = {};
    try {
      partB4 = await page.evaluate(() => {
        const widgetSels = ['.bento-item', '.widget', '.card', '[class*="widget"]'];
        const measurements = [];
        for (const s of widgetSels) {
          const els = document.querySelectorAll(s);
          els.forEach((el, i) => {
            if (i < 5 && el.getBoundingClientRect().width > 0) {
              const cs = getComputedStyle(el);
              measurements.push({
                sel: s + '[' + i + ']',
                paddingTop: cs.paddingTop,
                paddingRight: cs.paddingRight,
                paddingBottom: cs.paddingBottom,
                paddingLeft: cs.paddingLeft,
              });
            }
          });
          if (measurements.length >= 3) break;
        }
        return measurements.length ? measurements : [{ _missing: true }];
      });
      console.log(`PART_B_4 widget padding (first 3):`);
      partB4.slice(0, 3).forEach(p => console.log(`  ${p.sel}: pad ${p.paddingTop} ${p.paddingRight} ${p.paddingBottom} ${p.paddingLeft}`));
    } catch (e) { console.log(`PART_B_4 ERROR: ${e.message}`); }

    // B5: alert chip styling
    let partB5 = {};
    try {
      partB5 = await page.evaluate(() => {
        const chipSels = ['.alert', '.chip', '.badge', '.alert-chip', '[class*="alert"]',
          '[class*="chip"]', '[class*="badge"]', '[class*="tag"]'];
        const chips = [];
        for (const s of chipSels) {
          const els = document.querySelectorAll(s);
          els.forEach((el, i) => {
            if (i < 4 && el.getBoundingClientRect().width > 0) {
              const cs = getComputedStyle(el);
              chips.push({
                sel: s + '[' + i + ']',
                bg: cs.backgroundColor,
                color: cs.color,
                borderRadius: cs.borderRadius,
                text: el.textContent.trim().slice(0, 30),
              });
            }
          });
          if (chips.length >= 2) break;
        }
        return chips.length ? chips : [{ _missing: true }];
      });
      console.log(`PART_B_5 alert chips: ${JSON.stringify(partB5[0])}`);
      if (partB5[1]) console.log(`PART_B_5 alert chips[1]: ${JSON.stringify(partB5[1])}`);
    } catch (e) { console.log(`PART_B_5 ERROR: ${e.message}`); }

    // B6: card title font-size, amount font-size
    let partB6 = {};
    try {
      partB6 = await page.evaluate(() => {
        const result = {};
        // Card/widget title font
        const titleSels = ['.wtitle', '.whead h2', '.whead h3', '.card-title', '.widget-title',
          'h2', 'h3', '[class*="title"]'];
        for (const s of titleSels) {
          const el = document.querySelector(s);
          if (el && el.getBoundingClientRect().width > 0) {
            const cs = getComputedStyle(el);
            result.titleFontSize = cs.fontSize;
            result.titleFontWeight = cs.fontWeight;
            result.titleSel = s;
            break;
          }
        }
        // Amount font
        const amtSels = ['.amount', '.value', '.big-number', '[class*="amount"]',
          '[class*="value"]', '[class*="balance"]'];
        for (const s of amtSels) {
          const el = document.querySelector(s);
          if (el && el.getBoundingClientRect().width > 0) {
            const cs = getComputedStyle(el);
            result.amountFontSize = cs.fontSize;
            result.amountFontWeight = cs.fontWeight;
            result.amountSel = s;
            break;
          }
        }
        return result;
      });
      console.log(`PART_B_6 title: ${partB6.titleFontSize} weight:${partB6.titleFontWeight} sel:${partB6.titleSel}`);
      console.log(`PART_B_6 amount: ${partB6.amountFontSize} weight:${partB6.amountFontWeight} sel:${partB6.amountSel}`);
    } catch (e) { console.log(`PART_B_6 ERROR: ${e.message}`); }

    // Store results
    allResults[key] = { partA1, partA2, partA3, partA4, partB1, partB2, partB3, partB4, partB5, partB6 };

    // Screenshot
    const shotName = `gx11_dashboard_${width}_${theme}.png`;
    const shotPath = path.join(SCREENSHOTS_DIR, shotName);
    await page.screenshot({ path: shotPath, fullPage: false });
    console.log(`SCREENSHOT: ${shotName}`);

    await context.close();
  }
}

await browser.close();

console.log('\n========== ALL DONE ==========');
console.log('Screenshots saved to e2e/screenshots/');
console.log('Viewports tested:', VIEWPORTS.join(', '));
console.log('Themes tested:', THEMES.join(', '));
