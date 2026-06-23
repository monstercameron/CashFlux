/**
 * gm_04_palette_gear.mjs — GM4 FlipPanel system + widget gear + command palette UX review
 *
 * Probes:
 *   (a) Command palette — Ctrl+K open, screenshot, type to filter, keyboard nav, ESC close.
 *   (b) Per-widget gear panel — hover a widget, click its gear button, inspect the
 *       FlipPanel settings back-face (B12; C11 empty-panel handling).
 *   (c) FlipPanel flip animation / backdrop — shared mechanics.
 *
 * Both themes (dark + light), both viewports (1280×900 + 768×1024).
 *
 * Run: node e2e/gm_04_palette_gear.mjs
 */
import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import fs from 'fs';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');

const BASE_URL = process.env.E2E_URL || 'http://127.0.0.1:8099';
const SS_DIR   = path.resolve('e2e/screenshots');
const DATA_DIR = path.resolve('e2e/screenshots');
if (!fs.existsSync(SS_DIR)) fs.mkdirSync(SS_DIR, { recursive: true });

const ss   = (name) => path.join(SS_DIR,   `gm_04_${name}.png`);
const data = (name) => path.join(DATA_DIR, `gm_04_${name}.json`);

const errors = [];

// ── boot helpers ──────────────────────────────────────────────────────────────

async function newPage(browser, vw, vh = 900) {
  const ctx  = await browser.newContext({ viewport: { width: vw, height: vh } });
  const page = await ctx.newPage();
  page.on('pageerror', e => errors.push(`[${vw}] ${e.message}`));
  return { ctx, page };
}

async function bootDark(browser, vw, vh) {
  const { ctx, page } = await newPage(browser, vw, vh);
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.evaluate(() => {
    const p = JSON.parse(localStorage.getItem('cashflux:prefs') || '{}');
    p.theme = 'dark';
    delete p.viewAsMember;
    localStorage.setItem('cashflux:prefs', JSON.stringify(p));
  });
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 30000 });
  await page.waitForTimeout(500);
  return { ctx, page };
}

async function bootLight(browser, vw, vh) {
  const { ctx, page } = await newPage(browser, vw, vh);
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.evaluate(() => {
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' }));
  });
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForFunction(
    () => document.documentElement.getAttribute('data-theme') === 'light',
    { timeout: 10000 }
  ).catch(() => {});
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 30000 });
  await page.waitForTimeout(500);
  return { ctx, page };
}

// Navigate to /dashboard and wait for bento grid
async function navDashboard(page) {
  await page.goto(BASE_URL + '/dashboard', { waitUntil: 'networkidle' });
  await page.waitForSelector('.bento, [data-widget]', { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(500);
}

// ── command palette helpers ───────────────────────────────────────────────────

async function openPalette(page) {
  await page.keyboard.press('Control+k');
  // Wait for the palette overlay to appear
  await page.waitForSelector('#cf-cmd-palette', { timeout: 8000 }).catch(async () => {
    // Try Meta+K (macOS style in browser)
    await page.keyboard.press('Meta+k');
    await page.waitForSelector('#cf-cmd-palette', { timeout: 5000 }).catch(() => {});
  });
  await page.waitForTimeout(300);
}

async function closePaletteEsc(page) {
  await page.keyboard.press('Escape');
  await page.waitForTimeout(300);
}

// ── gear (widget settings FlipPanel) helpers ──────────────────────────────────

async function openWidgetGear(page) {
  // Hover over the first widget to reveal the gear-inline button
  const widget = page.locator('[data-widget]').first();
  const count = await widget.count();
  if (count === 0) return false;

  await widget.hover();
  await page.waitForTimeout(300);

  // The gear button is .gear-inline inside the hovered widget
  const gear = widget.locator('button.gear-inline').first();
  const gearVisible = await gear.isVisible().catch(() => false);
  if (!gearVisible) {
    // Try clicking any gear-inline
    const anyGear = page.locator('button.gear-inline').first();
    if (await anyGear.count() > 0) {
      await anyGear.click();
    } else {
      return false;
    }
  } else {
    await gear.click();
  }

  // Wait for the FlipPanel back-face to appear (animated flip)
  await page.waitForSelector('.flip-backdrop, .flip-wrap', { timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(800); // let flip animation complete
  return true;
}

async function closeGearPanel(page) {
  // Try the set-close button (×)
  const closeBtn = page.locator('.set-close').first();
  if (await closeBtn.isVisible().catch(() => false)) {
    await closeBtn.click();
  } else {
    await page.keyboard.press('Escape');
  }
  await page.waitForTimeout(400);
}

// ── DOM audits ────────────────────────────────────────────────────────────────

async function auditPalette(page) {
  return page.evaluate(() => {
    const ov  = document.getElementById('cf-cmd-palette');
    const inp = document.getElementById('cf-cmd-input');
    const lst = document.getElementById('cf-cmd-list');

    if (!ov) return { found: false };

    const ovStyle  = getComputedStyle(ov);
    const inpStyle = inp ? getComputedStyle(inp) : null;

    // Card = the first direct child div
    const card = ov.querySelector('div');
    const cardStyle = card ? getComputedStyle(card) : null;

    // Count result rows and group headers
    const rows    = lst ? lst.querySelectorAll('[data-cmd-row]') : [];
    const headers = lst ? lst.querySelectorAll('[role="presentation"]') : [];

    // Is palette visible?
    const display = ovStyle.display;

    // Focus: is the input focused?
    const inputFocused = inp ? document.activeElement === inp : false;

    // ARIA: does the card/overlay have role attributes?
    const cardRole = card ? card.getAttribute('role') : null;
    const hasAriaModal = card ? card.getAttribute('aria-modal') : null;

    // Backdrop color
    const backdropBg = ovStyle.backgroundColor;

    return {
      found:         true,
      display,
      inputFocused,
      inputPlaceholder: inp ? inp.getAttribute('placeholder') : null,
      inputAriaLabel:   inp ? inp.getAttribute('aria-label')  : null,
      backdropBg,
      cardBg:           cardStyle ? cardStyle.backgroundColor : null,
      cardBorder:       cardStyle ? cardStyle.border : null,
      cardBorderRadius: cardStyle ? cardStyle.borderRadius : null,
      rowCount:         rows.length,
      headerCount:      headers.length,
      cardRole,
      hasAriaModal,
      cardHasRole:      !!cardRole,
      ovZIndex:         ovStyle.zIndex,
    };
  });
}

async function auditGearPanel(page) {
  return page.evaluate(() => {
    const backdrop = document.querySelector('.flip-backdrop');
    const wrap     = document.querySelector('.flip-wrap');
    const inner    = document.querySelector('.flip-inner');
    const back     = document.querySelector('.flip-back');
    const setH     = document.querySelector('.set-h');
    const setBody  = document.querySelector('.set-body');
    const setFoot  = document.querySelector('.set-foot');

    if (!backdrop) return { found: false };

    const bdStyle   = getComputedStyle(backdrop);
    const backStyle = back ? getComputedStyle(back) : null;
    const wrapStyle = wrap ? getComputedStyle(wrap) : null;

    // ARIA
    const wrapRole      = wrap ? wrap.getAttribute('role') : null;
    const wrapAriaModal = wrap ? wrap.getAttribute('aria-modal') : null;
    const wrapAriaLabel = wrap ? wrap.getAttribute('aria-label') : null;

    // Flip state
    const isFlipped = inner ? inner.classList.contains('flipped') : false;
    const isShown   = backdrop.classList.contains('show');

    // Title
    const titleEl  = setH ? setH.querySelector('h3') : null;
    const titleText = titleEl ? titleEl.textContent.trim() : null;

    // Footer buttons
    const footBtns = setFoot ? [...setFoot.querySelectorAll('button')] : [];
    const footLabels = footBtns.map(b => b.textContent.trim());

    // Body content
    const bodyText   = setBody ? setBody.textContent.trim() : null;
    const bodyIsEmpty = !bodyText || bodyText.length < 5;

    // Controls in the body
    const inputs    = setBody ? setBody.querySelectorAll('input, select, textarea') : [];
    const toggles   = setBody ? setBody.querySelectorAll('[class*="toggle"], input[type="checkbox"]') : [];

    // Colors
    const bdBg          = bdStyle.backgroundColor;
    const bdBdFilter    = bdStyle.backdropFilter;
    const backBg        = backStyle ? backStyle.backgroundColor : null;

    // Close button
    const closeBtn = document.querySelector('.set-close');
    const closeBtnExists = !!closeBtn;

    return {
      found:        true,
      isShown,
      isFlipped,
      wrapRole,
      wrapAriaModal,
      wrapAriaLabel,
      titleText,
      footLabels,
      bodyIsEmpty,
      inputCount:  inputs.length,
      toggleCount: toggles.length,
      bdBg,
      bdBdFilter,
      backBg,
      closeBtnExists,
      wrapWidth:  wrapStyle ? wrapStyle.width : null,
      wrapHeight: wrapStyle ? wrapStyle.height : null,
    };
  });
}

async function auditPaletteColors(page, theme) {
  return page.evaluate((theme) => {
    const ov  = document.getElementById('cf-cmd-palette');
    const inp = document.getElementById('cf-cmd-input');
    const lst = document.getElementById('cf-cmd-list');
    if (!ov || !inp || !lst) return { found: false };

    const firstRow = lst.querySelector('[data-cmd-row]');
    const selRow   = lst.querySelector('[data-cmd-row][style*="var(--hover"]');

    const inpStyle     = getComputedStyle(inp);
    const firstRowSt   = firstRow ? getComputedStyle(firstRow) : null;

    return {
      theme,
      found:           true,
      inpBg:           inpStyle.backgroundColor,
      inpColor:        inpStyle.color,
      inpFontSize:     inpStyle.fontSize,
      firstRowBg:      firstRowSt ? firstRowSt.backgroundColor : null,
      firstRowColor:   firstRowSt ? firstRowSt.color : null,
      selRowFound:     !!selRow,
    };
  }, theme);
}

// ── main probe ────────────────────────────────────────────────────────────────

async function probe(browser, theme, vw, vh) {
  const label = `${theme}_${vw}`;
  console.log(`\n=== ${label} ===`);

  const boot   = theme === 'light' ? bootLight : bootDark;
  const { ctx, page } = await boot(browser, vw, vh);

  try {
    // Navigate to dashboard
    await navDashboard(page);

    // ── (A) Command palette ──────────────────────────────────────────────────

    // 1. Open palette
    await openPalette(page);

    const paletteAudit = await auditPalette(page);
    console.log('  palette open:', JSON.stringify(paletteAudit, null, 2));

    await page.screenshot({ path: ss(`palette_open_${label}`), fullPage: false });
    console.log(`  shot: gm_04_palette_open_${label}.png`);

    // 2. Color audit while open
    const colorAudit = await auditPaletteColors(page, theme);
    console.log('  palette colors:', JSON.stringify(colorAudit));

    // 3. Type to filter
    if (paletteAudit.found && paletteAudit.display !== 'none') {
      await page.fill('#cf-cmd-input', 'acc');
      await page.waitForTimeout(300);
      await page.screenshot({ path: ss(`palette_filter_${label}`), fullPage: false });
      console.log(`  shot: gm_04_palette_filter_${label}.png`);

      // 4. Arrow-down navigation
      await page.keyboard.press('ArrowDown');
      await page.waitForTimeout(150);
      await page.keyboard.press('ArrowDown');
      await page.waitForTimeout(150);
      await page.screenshot({ path: ss(`palette_nav_${label}`), fullPage: false });
      console.log(`  shot: gm_04_palette_nav_${label}.png`);

      // 5. ESC to close
      await closePaletteEsc(page);
      const paletteHidden = await page.evaluate(() => {
        const ov = document.getElementById('cf-cmd-palette');
        return ov ? getComputedStyle(ov).display === 'none' : true;
      });
      console.log('  palette hidden after ESC:', paletteHidden);
    }

    // ── (B) Widget gear panel ────────────────────────────────────────────────

    // Re-navigate to dashboard (fresh state)
    await navDashboard(page);

    // Screenshot dashboard before gear open (shows gear visibility behavior)
    await page.screenshot({ path: ss(`dashboard_${label}`), fullPage: false });
    console.log(`  shot: gm_04_dashboard_${label}.png`);

    // Hover widget and open gear
    const gearOpened = await openWidgetGear(page);
    console.log('  gear opened:', gearOpened);

    if (gearOpened) {
      const gearAudit = await auditGearPanel(page);
      console.log('  gear panel:', JSON.stringify(gearAudit, null, 2));

      await page.screenshot({ path: ss(`gear_open_${label}`), fullPage: false });
      console.log(`  shot: gm_04_gear_open_${label}.png`);

      // Close via ESC
      await page.keyboard.press('Escape');
      await page.waitForTimeout(400);
      const gearGone = await page.evaluate(() => !document.querySelector('.flip-backdrop'));
      console.log('  gear gone after ESC:', gearGone);

      await page.screenshot({ path: ss(`gear_closed_${label}`), fullPage: false });
      console.log(`  shot: gm_04_gear_closed_${label}.png`);
    } else {
      // Document the attempt
      await page.screenshot({ path: ss(`gear_attempt_${label}`), fullPage: false });
      console.log(`  shot: gm_04_gear_attempt_${label}.png (gear could not be opened)`);
    }

    // ── (C) FlipPanel at 768 — if not already at 768, still covered by the vw param ──

    // Save DOM audit data
    const domData = {
      theme, vw,
      paletteAudit,
      colorAudit,
      gearOpened,
    };
    fs.writeFileSync(data(`dom_${label}`), JSON.stringify(domData, null, 2));
    console.log(`  data: gm_04_dom_${label}.json`);

  } catch (err) {
    errors.push(`[${label}] ${err.message}`);
    console.error(`  ERROR: ${err.message}`);
    await page.screenshot({ path: ss(`error_${label}`), fullPage: false }).catch(() => {});
  }

  await ctx.close();
}

// ── run ───────────────────────────────────────────────────────────────────────

const browser = await chromium.launch({ headless: true });

// Check server is up
const ok = await (async () => {
  try {
    const r = await fetch(BASE_URL);
    return r.status === 200;
  } catch { return false; }
})();
if (!ok) {
  console.error(`FATAL: ${BASE_URL} is not responding. Start gwc dev first.`);
  await browser.close();
  process.exit(1);
}
console.log(`Server OK: ${BASE_URL}`);

// Run all four combinations sequentially
await probe(browser, 'dark',  1280, 900);
await probe(browser, 'dark',   768, 1024);
await probe(browser, 'light', 1280, 900);
await probe(browser, 'light',  768, 1024);

await browser.close();

if (errors.length > 0) {
  console.error('\nPage errors collected:');
  errors.forEach(e => console.error(' ', e));
} else {
  console.log('\nNo page errors.');
}

console.log('\nDone. Exit 0.');
