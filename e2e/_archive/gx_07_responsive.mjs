/**
 * GX7 — Ultra-wide & portrait responsive probe
 * Tests layout at 2560×1440, 1920×1080, 600×900, 400×800
 * Pages: /dashboard, /transactions, /budgets, /goals, /reports + add modal
 * Measures: content width, scrollWidth overflow, bento columns, topbar height, line length
 */

import { chromium } from 'playwright';
import { writeFileSync, mkdirSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const SCREENSHOTS = join(__dirname, 'screenshots');
const BASE_URL = 'http://localhost:8080';

mkdirSync(SCREENSHOTS, { recursive: true });

const VIEWPORTS = [
  { label: '2560x1440', width: 2560, height: 1440, tag: '2560' },
  { label: '1920x1080', width: 1920, height: 1080, tag: '1920' },
  { label: '600x900',   width: 600,  height: 900,  tag: '0600' },
  { label: '400x800',   width: 400,  height: 800,  tag: '0400' },
];

const PAGES = [
  { path: '/',             label: 'dashboard',     tag: 'dash' },
  { path: '/transactions', label: 'transactions',  tag: 'txn'  },
  { path: '/budgets',      label: 'budgets',       tag: 'budg' },
  { path: '/goals',        label: 'goals',         tag: 'goal' },
  { path: '/reports',      label: 'reports',       tag: 'rpt'  },
];

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
  }, theme);
  await page.reload({ waitUntil: 'networkidle' });
  try {
    await page.waitForFunction(
      (t) => document.documentElement.getAttribute('data-theme') === t,
      theme,
      { timeout: 5000 }
    );
  } catch (_) { /* theme attr may not be present */ }
}

async function waitForApp(page) {
  // Wait for wasm app to hydrate — look for nav rail or main content
  await page.waitForSelector('aside.rail, [class*="rail"], main, #app > *', { timeout: 10000 });
  // Small settle delay for wasm rendering
  await page.waitForTimeout(600);
}

async function measure(page) {
  return await page.evaluate(() => {
    // Content width: .page element or main > first child
    const pageEl = document.querySelector('.page') ||
                   document.querySelector('main > div') ||
                   document.querySelector('main');
    const contentWidth = pageEl ? pageEl.getBoundingClientRect().width : null;

    // Scroll overflow
    const docScrollW = document.documentElement.scrollWidth;
    const docClientW = document.documentElement.clientWidth;
    const bodyScrollW = document.body.scrollWidth;
    const overflow = Math.max(docScrollW, bodyScrollW) - docClientW;

    // Topbar height
    const topbar = document.querySelector('.topbar, [class*="topbar"]');
    const topbarH = topbar ? topbar.getBoundingClientRect().height : null;

    // Rail width
    const rail = document.querySelector('aside.rail, .rail');
    const railW = rail ? rail.getBoundingClientRect().width : null;

    // Bento columns
    const bento = document.querySelector('.bento');
    let bentoColumns = null;
    if (bento) {
      const style = window.getComputedStyle(bento);
      const cols = style.getPropertyValue('grid-template-columns');
      if (cols && cols !== 'none') {
        bentoColumns = cols.trim().split(/\s+(?=\d|\()/).filter(Boolean).length;
      }
    }

    // Longest text line (sample visible text nodes under .page)
    let maxLineLen = 0;
    const walker = document.createTreeWalker(
      pageEl || document.body,
      NodeFilter.SHOW_TEXT,
      null
    );
    let node;
    while ((node = walker.nextNode())) {
      const text = node.textContent.trim();
      if (text.length > maxLineLen) maxLineLen = text.length;
    }

    // Table: does txn-table have horizontal scroll?
    const table = document.querySelector('.txn-table');
    let tableOverflow = null;
    if (table) {
      tableOverflow = table.scrollWidth - table.clientWidth;
    }

    // Main max-width CSS value
    const mainEl = document.querySelector('main');
    const mainStyle = mainEl ? window.getComputedStyle(mainEl) : null;
    const mainMaxW = mainStyle ? mainStyle.maxWidth : null;

    return {
      contentWidth: contentWidth ? Math.round(contentWidth) : null,
      overflow: Math.round(overflow),
      docScrollW, docClientW: Math.round(docClientW),
      topbarH: topbarH ? Math.round(topbarH) : null,
      railW: railW ? Math.round(railW) : null,
      bentoColumns,
      maxLineLen,
      tableOverflow,
      mainMaxW,
      vw: window.innerWidth,
      vh: window.innerHeight,
    };
  });
}

async function run() {
  const browser = await chromium.launch({ headless: true });
  const results = [];

  for (const vp of VIEWPORTS) {
    console.log(`\n=== VIEWPORT ${vp.label} ===`);
    const context = await browser.newContext({
      viewport: { width: vp.width, height: vp.height },
    });
    const page = await context.newPage();

    // Navigate to app first to set theme
    await page.goto(BASE_URL + '/', { waitUntil: 'networkidle', timeout: 15000 });
    await waitForApp(page);

    // Dark theme (default)
    const theme = 'dark';

    for (const pg of PAGES) {
      console.log(`  ${pg.label}...`);
      try {
        await page.goto(BASE_URL + pg.path, { waitUntil: 'networkidle', timeout: 15000 });
        await waitForApp(page);
        await page.waitForTimeout(400);

        const m = await measure(page);
        const tag = `gx07_${vp.tag}_${pg.tag}_${theme}`;
        const ssPath = join(SCREENSHOTS, tag + '.png');
        await page.screenshot({ path: ssPath, fullPage: false });

        const row = { viewport: vp.label, page: pg.label, theme, ...m, screenshot: tag + '.png' };
        results.push(row);
        console.log(`    content=${m.contentWidth}px overflow=${m.overflow}px topbar=${m.topbarH}px rail=${m.railW}px bento=${m.bentoColumns}cols ss=${tag}.png`);
      } catch (e) {
        console.log(`    SKIP: ${e.message.slice(0, 80)}`);
        results.push({ viewport: vp.label, page: pg.label, theme, error: e.message.slice(0, 80) });
      }
    }

    // Test add modal at this viewport
    try {
      console.log(`  add-modal...`);
      await page.goto(BASE_URL + '/', { waitUntil: 'networkidle', timeout: 15000 });
      await waitForApp(page);

      // Try clicking the +Add button (topbar)
      const addBtn = await page.$('.menu-btn, button[title*="Add"], button[aria-label*="Add"]');
      if (addBtn) {
        await addBtn.click();
        await page.waitForTimeout(400);
      } else {
        // Try keyboard shortcut or find any button with "Add" text
        const btns = await page.$$('button');
        for (const btn of btns) {
          const txt = await btn.textContent();
          if (txt && txt.trim().toLowerCase().includes('add')) {
            await btn.click();
            await page.waitForTimeout(300);
            break;
          }
        }
      }

      // Look for a modal/dialog
      const modal = await page.$('.flip-wrap, dialog, [role="dialog"], .modal');
      if (modal) {
        const modalMetrics = await page.evaluate(() => {
          const m = document.querySelector('.flip-wrap, dialog, [role="dialog"], .modal');
          if (!m) return null;
          const r = m.getBoundingClientRect();
          return { width: Math.round(r.width), height: Math.round(r.height), left: Math.round(r.left), right: Math.round(r.right) };
        });
        console.log(`    modal: ${JSON.stringify(modalMetrics)}`);

        const tag = `gx07_${vp.tag}_modal_${theme}`;
        const ssPath = join(SCREENSHOTS, tag + '.png');
        await page.screenshot({ path: ssPath, fullPage: false });
        results.push({ viewport: vp.label, page: 'modal', theme, modal: modalMetrics, screenshot: tag + '.png' });
        console.log(`    ss=${tag}.png`);
      } else {
        console.log(`    no modal opened`);
        results.push({ viewport: vp.label, page: 'modal', theme, note: 'no modal found' });
      }
    } catch (e) {
      console.log(`    modal SKIP: ${e.message.slice(0, 80)}`);
    }

    // Light theme spot-check at 400px only
    if (vp.width === 400) {
      console.log(`  [light spot-check]`);
      try {
        await page.goto(BASE_URL + '/', { waitUntil: 'networkidle', timeout: 15000 });
        await waitForApp(page);
        await setTheme(page, 'light');
        await waitForApp(page);

        const m = await measure(page);
        const tag = `gx07_${vp.tag}_dash_light`;
        await page.screenshot({ path: join(SCREENSHOTS, tag + '.png'), fullPage: false });
        results.push({ viewport: vp.label, page: 'dashboard', theme: 'light', ...m, screenshot: tag + '.png' });
        console.log(`    light: content=${m.contentWidth}px overflow=${m.overflow}px ss=${tag}.png`);

        // Also txn light
        await page.goto(BASE_URL + '/transactions', { waitUntil: 'networkidle', timeout: 15000 });
        await waitForApp(page);
        await page.waitForTimeout(400);
        const m2 = await measure(page);
        const tag2 = `gx07_${vp.tag}_txn_light`;
        await page.screenshot({ path: join(SCREENSHOTS, tag2 + '.png'), fullPage: false });
        results.push({ viewport: vp.label, page: 'transactions', theme: 'light', ...m2, screenshot: tag2 + '.png' });
        console.log(`    light txn: content=${m2.contentWidth}px overflow=${m2.overflow}px ss=${tag2}.png`);
      } catch (e) {
        console.log(`    light SKIP: ${e.message.slice(0, 80)}`);
      }
    }

    await context.close();
  }

  // Save measurements JSON
  const jsonPath = join(SCREENSHOTS, 'gx07_measurements.json');
  writeFileSync(jsonPath, JSON.stringify(results, null, 2));
  console.log(`\nMeasurements saved to ${jsonPath}`);
  console.log(`Total screenshots: ${results.filter(r => r.screenshot).length}`);

  await browser.close();
}

run().catch(e => { console.error(e); process.exit(1); });
