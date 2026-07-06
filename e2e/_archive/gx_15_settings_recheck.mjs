/**
 * GX15 — Settings panel re-check (post GX14/GM1)
 * "Make It Mine, Revisited"
 *
 * Part A: Regression confirm for G21/GM1 light-mode failures
 * Part B: Residual blemishes audit
 *
 * Usage: node e2e/gx_15_settings_recheck.mjs
 */

import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOTS_DIR = path.join(__dirname, 'screenshots');
const BASE_URL = 'http://localhost:8080';

if (!fs.existsSync(SCREENSHOTS_DIR)) {
  fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
}

function pass(label, value, note = '') {
  console.log(`  ✓ PASS  ${label}: ${value}${note ? '  (' + note + ')' : ''}`);
}
function fail(label, value, note = '') {
  console.log(`  ✗ FAIL  ${label}: ${value}${note ? '  (' + note + ')' : ''}`);
}
function info(label, value) {
  console.log(`  ℹ       ${label}: ${value}`);
}

async function getComputedStyle(page, selector, prop) {
  return page.evaluate(
    ([sel, p]) => {
      const el = document.querySelector(sel);
      if (!el) return '__NOT_FOUND__';
      return window.getComputedStyle(el).getPropertyValue(p).trim();
    },
    [selector, prop]
  );
}

async function openSettings(page) {
  // Open settings by clicking button.hh (household card at rail bottom)
  const hhBtn = await page.$('button.hh');
  if (!hhBtn) {
    console.log('  ! button.hh not found — trying fallback selectors');
    // try gear icon or settings button
    const fallbacks = ['.rail-foot button', 'button[aria-label*="etting"]', 'button.gear'];
    for (const sel of fallbacks) {
      const el = await page.$(sel);
      if (el) {
        console.log(`  ! Using fallback: ${sel}`);
        await el.click();
        return true;
      }
    }
    return false;
  }
  await hhBtn.click();
  return true;
}

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
  }, theme);
  await page.reload({ waitUntil: 'networkidle' });
  // Wait for theme to apply
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute('data-theme') === t,
    theme,
    { timeout: 10000 }
  ).catch(() => {
    console.log(`  ! Warning: data-theme="${theme}" not detected on <html> after reload`);
  });
}

async function waitForSettings(page) {
  // Wait for settings panel to be visible
  await page.waitForSelector('.set-face, .flip-face', { timeout: 5000 }).catch(() => {
    console.log('  ! Settings panel selectors not found');
  });
  // Small settle wait
  await page.waitForTimeout(400);
}

async function runLightCheck(page) {
  console.log('\n=== PART A — REGRESSION CONFIRM (light theme) ===');

  // 1. Toggle-row labels
  const toggleColor = await getComputedStyle(page, '.toggle-row span', 'color');
  if (toggleColor === '__NOT_FOUND__') {
    info('toggle-row span color', 'element not found');
  } else {
    // Expected: dark (#1c1c1e = rgb(28,28,30)), NOT near-white
    const isWhiteOnWhite = toggleColor.includes('244') || toggleColor.includes('255,255,255');
    if (!isWhiteOnWhite && toggleColor !== 'rgba(0, 0, 0, 0)') {
      pass('.toggle-row span color', toggleColor, 'dark text — not white-on-white');
    } else {
      fail('.toggle-row span color', toggleColor, 'REGRESSION: near-white on white — G21 defect not fixed');
    }
  }

  // 2. Panel backdrop
  const backdropBg = await getComputedStyle(page, '.flip-backdrop', 'background-color');
  if (backdropBg === '__NOT_FOUND__') {
    info('.flip-backdrop background-color', 'element not found');
  } else {
    // Expected: warm-white rgba(239,237,232,0.75), NOT dark (rgba(4,4,6,...))
    const isDark = backdropBg.includes('4, 4, 6') || backdropBg.includes('0, 0, 0, 0.6');
    if (!isDark) {
      pass('.flip-backdrop background-color', backdropBg, 'warm-white — not dark');
    } else {
      fail('.flip-backdrop background-color', backdropBg, 'REGRESSION: dark backdrop on light — G21 defect not fixed');
    }
  }

  // 3. Panel face
  const faceBg = await getComputedStyle(page, '.flip-face, .set-face', 'background-color');
  if (faceBg === '__NOT_FOUND__') {
    // try alternatives
    const altFace = await getComputedStyle(page, '.set-face', 'background-color');
    info('.set-face background-color', altFace);
  } else {
    const isWhite = faceBg.includes('255, 255, 255') || faceBg.includes('rgb(255, 255, 255)');
    if (isWhite) {
      pass('.flip-face/.set-face background-color', faceBg, 'white panel');
    } else {
      fail('.flip-face/.set-face background-color', faceBg, 'not white — check if dark bleed');
    }
  }

  // 4. Section labels
  const labelColor = await getComputedStyle(page, '.set-label', 'color');
  if (labelColor === '__NOT_FOUND__') {
    info('.set-label color', 'element not found');
  } else {
    // Expected: dark readable (#3c3c43 = rgb(60,60,67))
    const isDark = !labelColor.includes('244') && !labelColor.includes('255,255,255');
    if (isDark) {
      pass('.set-label color', labelColor, 'readable dark label');
    } else {
      fail('.set-label color', labelColor, 'near-white label on light');
    }
  }

  // 5. Inputs
  const inputBg = await getComputedStyle(page, '.set-input', 'background-color');
  const inputColor = await getComputedStyle(page, '.set-input', 'color');
  info('.set-input background-color', inputBg);
  info('.set-input color', inputColor);
  if (inputBg !== '__NOT_FOUND__') {
    const bgIsLight = inputBg.includes('255, 255, 255') || inputBg.includes('rgb(255');
    bgIsLight ? pass('.set-input bg', inputBg, 'white bg') : fail('.set-input bg', inputBg, 'not white');
    const colorIsDark = inputColor && !inputColor.includes('244') && !inputColor.includes('255,255,255');
    colorIsDark ? pass('.set-input color', inputColor, 'dark text') : fail('.set-input color', inputColor, 'light text — may be unreadable');
  }

  // 6. Panel title
  const titleColor = await getComputedStyle(page, '.set-h h3', 'color');
  if (titleColor === '__NOT_FOUND__') {
    info('.set-h h3 color', 'element not found');
  } else {
    const isDark = !titleColor.includes('244') && !titleColor.includes('255,255,255');
    isDark ? pass('.set-h h3 color', titleColor, 'dark title') : fail('.set-h h3 color', titleColor, 'REGRESSION: near-white title on light');
  }
}

async function runPartB(page) {
  console.log('\n=== PART B — RESIDUAL BLEMISHES ===');

  // 1. Count sections
  const sectionCount = await page.evaluate(() => {
    return document.querySelectorAll('.set-section').length;
  });
  info('.set-section count', sectionCount);

  // 1b. Is nav/jump present?
  const jumpNavCount = await page.evaluate(() => {
    return document.querySelectorAll('.set-nav, .jump-nav, [class*="jump"]').length;
  });
  info('jump-nav elements (.set-nav/.jump-nav/[class*=jump])', jumpNavCount);
  jumpNavCount > 0 ? pass('Jump navigation present', jumpNavCount + ' elements') : fail('Jump navigation', 'MISSING — S8 not implemented');

  // 2. Section order
  const sectionLabels = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('.set-label')).map(el => el.textContent.trim());
  });
  info('Section labels in order', JSON.stringify(sectionLabels));

  // 3. Import buttons
  const importBtns = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('button')).filter(b =>
      b.textContent.trim().toLowerCase().includes('import')
    ).map(b => b.textContent.trim());
  });
  info('Import button count', importBtns.length);
  info('Import button texts', JSON.stringify(importBtns));

  // 4. Save button — click and check for toast/confirmation
  const saveBtn = await page.$('.set-btn.save, button.save, [class*="save"]');
  if (saveBtn) {
    await saveBtn.click();
    await page.waitForTimeout(600);
    // Check for toast
    const toastText = await page.evaluate(() => {
      const toasts = document.querySelectorAll('.toast, [class*="toast"], [role="status"], [role="alert"]');
      return Array.from(toasts).map(t => t.textContent.trim()).filter(Boolean);
    });
    info('Save button click — toasts/alerts', JSON.stringify(toastText));
    toastText.length > 0 ? pass('Save feedback', 'toast/alert appeared: ' + toastText[0]) : fail('Save feedback', 'no toast/alert detected after save');
  } else {
    fail('Save button', 'not found (.set-btn.save)');
  }

  // 5. Overflow at 768px
  await page.setViewportSize({ width: 768, height: 900 });
  await page.waitForTimeout(300);
  const overflowResult = await page.evaluate(() => {
    const panel = document.querySelector('.set-face, .flip-face');
    if (!panel) return { found: false };
    return {
      found: true,
      scrollWidth: panel.scrollWidth,
      clientWidth: panel.clientWidth,
      overflows: panel.scrollWidth > panel.clientWidth,
    };
  });
  info('Panel at 768px (scrollWidth/clientWidth)', overflowResult.found
    ? `${overflowResult.scrollWidth}/${overflowResult.clientWidth} overflows=${overflowResult.overflows}`
    : 'panel not found');
  if (overflowResult.found) {
    overflowResult.overflows
      ? fail('Panel overflow at 768px', `scrollWidth(${overflowResult.scrollWidth}) > clientWidth(${overflowResult.clientWidth})`)
      : pass('Panel overflow at 768px', 'no overflow');
  }
  // Reset viewport
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.waitForTimeout(300);

  // 6. Heading semantics — are section labels div or h4?
  const labelTagNames = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('.set-label')).slice(0, 5).map(el => el.tagName.toLowerCase());
  });
  info('.set-label element tag names (first 5)', JSON.stringify(labelTagNames));
  const hasH4 = labelTagNames.some(t => t === 'h4');
  const allDiv = labelTagNames.every(t => t === 'div');
  if (hasH4) {
    pass('Section label semantics', 'h4 elements — correct heading hierarchy');
  } else if (allDiv) {
    fail('Section label semantics', 'all <div> — missing heading semantics [GO-STRUCTURAL]');
  } else {
    info('Section label semantics', JSON.stringify(labelTagNames));
  }
}

async function takeScreenshots(page, browser) {
  console.log('\n=== SCREENSHOTS ===');

  const shots = [
    { theme: 'light', width: 1280, name: 'settings_light_1280.png' },
    { theme: 'dark', width: 1280, name: 'settings_dark_1280.png' },
    { theme: 'light', width: 768, name: 'settings_light_768.png' },
    { theme: 'dark', width: 768, name: 'settings_dark_768.png' },
  ];

  for (const shot of shots) {
    await setTheme(page, shot.theme);
    await page.setViewportSize({ width: shot.width, height: 900 });
    await openSettings(page);
    await waitForSettings(page);
    const screenshotPath = path.join(SCREENSHOTS_DIR, shot.name);
    await page.screenshot({ path: screenshotPath, fullPage: false });
    console.log(`  Saved: ${shot.name}`);
  }
}

async function main() {
  console.log('GX15 — Settings panel re-check (post GX14/GM1)');
  console.log('================================================');
  console.log(`Base URL: ${BASE_URL}`);

  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();

  try {
    // Navigate to app
    await page.goto(BASE_URL, { waitUntil: 'networkidle', timeout: 15000 });
    console.log(`\nLoaded: ${page.url()}`);

    // Set light theme
    await setTheme(page, 'light');
    const currentTheme = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    info('data-theme after setItem + reload', currentTheme || '(none set on html)');

    // Open settings panel
    const opened = await openSettings(page);
    if (!opened) {
      console.log('ERROR: Could not open settings panel — no known trigger button found');
    } else {
      await waitForSettings(page);

      // Part A
      await runLightCheck(page);

      // Part B
      await runPartB(page);
    }

    // Screenshots — reopen for each theme
    await takeScreenshots(page, browser);

  } catch (err) {
    console.error('\nFATAL:', err.message);
  } finally {
    await browser.close();
  }

  console.log('\nGX15 script complete.');
}

main();
