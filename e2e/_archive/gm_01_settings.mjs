/**
 * gm_01_settings.mjs — GM1 Settings modal UX review
 * Opens the Settings FlipPanel via button.hh, screenshots at 1280+768 in dark+light.
 * Inspects modal structure + computed colors of backdrop/face/toggles/inputs.
 * Run: node e2e/gm_01_settings.mjs
 */
import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import fs from 'fs';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = require('playwright');

const BASE_URL = process.env.E2E_URL || 'http://127.0.0.1:8099';
const SS_DIR = path.resolve('e2e/screenshots');
const DATA_DIR = path.resolve('e2e/screenshots');
if (!fs.existsSync(SS_DIR)) fs.mkdirSync(SS_DIR, { recursive: true });

function ssPath(name) { return path.join(SS_DIR, name); }
function dataPath(name) { return path.join(DATA_DIR, name); }

async function hardReload(page) {
  await page.evaluate(() => window.location.reload());
  await page.waitForLoadState('networkidle');
}

async function bootDark(page) {
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.evaluate(() => {
    const p = JSON.parse(localStorage.getItem('cashflux:prefs') || '{}');
    p.theme = 'dark';
    localStorage.setItem('cashflux:prefs', JSON.stringify(p));
  });
  await hardReload(page);
}

async function bootLight(page) {
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' })));
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 8000 });
}

async function openSettings(page) {
  // Click the household card button at rail bottom
  const hh = page.locator('button.hh').first();
  await hh.waitFor({ state: 'visible', timeout: 8000 });
  await hh.click();
  // Wait for panel to appear
  await page.waitForSelector('.set-label', { timeout: 8000 });
  // Brief wait for flip animation
  await page.waitForTimeout(650);
}

async function closeSettings(page) {
  // Click the close button (×) if visible, else press Escape
  const closeBtn = page.locator('.set-close').first();
  if (await closeBtn.isVisible()) {
    await closeBtn.click();
  } else {
    await page.keyboard.press('Escape');
  }
  await page.waitForTimeout(400);
}

async function auditModal(page, tag) {
  return await page.evaluate((t) => {
    const backdrop = document.querySelector('.flip-backdrop');
    const flipWrap = document.querySelector('.flip-wrap');
    const flipInner = document.querySelector('.flip-inner');
    const flipFace = document.querySelector('.flip-face, .set-face');
    const setBody = document.querySelector('.set-body');
    const toggleRows = Array.from(document.querySelectorAll('.toggle-row span'));
    const setLabels = Array.from(document.querySelectorAll('.set-label'));
    const inputs = Array.from(document.querySelectorAll('input'));
    const selects = Array.from(document.querySelectorAll('select'));
    const labels = Array.from(document.querySelectorAll('label'));
    const dataBtns = Array.from(document.querySelectorAll('.data-btn'));
    const importBtns = dataBtns.filter(b => b.textContent.trim().toLowerCase().startsWith('import'));
    const switches = Array.from(document.querySelectorAll('.switch'));
    const swatches = Array.from(document.querySelectorAll('.swatch'));
    const memberChips = Array.from(document.querySelectorAll('.member-chip'));
    const setFoot = document.querySelector('.set-foot');
    const saveBtn = document.querySelector('.set-btn.save');
    const cancelBtn = document.querySelector('.set-btn.cancel');
    const closeBtn = document.querySelector('.set-close');

    const cs = (el) => el ? window.getComputedStyle(el) : null;

    // overflow check
    const allEls = Array.from(document.querySelectorAll('.flip-face *, .set-face *'));
    const overflowEls = allEls.filter(el => {
      const r = el.getBoundingClientRect();
      return r.right > window.innerWidth + 2 || r.bottom > window.innerHeight + 2;
    });

    // Aria checks
    const passwordInputs = inputs.filter(i => i.type === 'password');
    const passwordAriaLabels = passwordInputs.map(i => i.getAttribute('aria-label') || i.getAttribute('title') || i.placeholder || '(none)');

    // aria-modal
    const ariaModal = backdrop ? backdrop.getAttribute('aria-modal') : null;
    const roleDialog = backdrop ? backdrop.getAttribute('role') : null;

    // set-label tags
    const setLabelTags = [...new Set(setLabels.map(el => el.tagName.toLowerCase()))];

    // section names
    const sectionNames = setLabels.map(el => el.textContent.trim());

    // flip-wrap dimensions
    let wrapDims = null;
    if (flipWrap) {
      const r = flipWrap.getBoundingClientRect();
      wrapDims = { width: Math.round(r.width), height: Math.round(r.height) };
    }

    // computed colors
    const backdropStyle = cs(backdrop);
    const faceStyle = cs(flipFace);
    const toggleSpanStyle = toggleRows.length > 0 ? cs(toggleRows[0]) : null;
    const setLabelStyle = setLabels.length > 0 ? cs(setLabels[0]) : null;
    const setBodyStyle = cs(setBody);

    // first toggle span color
    const toggleSpanColor = toggleSpanStyle ? toggleSpanStyle.color : null;
    const toggleSpanBg = toggleSpanStyle ? toggleSpanStyle.backgroundColor : null;

    // backdrop bg
    const backdropBg = backdropStyle ? backdropStyle.backgroundColor : null;
    // face bg
    const faceBg = faceStyle ? faceStyle.backgroundColor : null;

    // set-label color
    const setLabelColor = setLabelStyle ? setLabelStyle.color : null;

    // check footer sticky
    let footPinned = false;
    if (setFoot) {
      const footR = setFoot.getBoundingClientRect();
      const bodyR = setBody ? setBody.getBoundingClientRect() : null;
      footPinned = !bodyR || (footR.bottom > bodyR.bottom + 2);
    }

    return {
      tag: t,
      panelFound: !!backdrop,
      flipWrapDims: wrapDims,
      sectionCount: setLabels.length,
      sectionNames,
      inputCount: inputs.length,
      passwordCount: passwordInputs.length,
      passwordAriaLabels,
      selectCount: selects.length,
      labelCount: labels.length,
      toggleCount: switches.length,
      toggleRowCount: toggleRows.length,
      dataBtnCount: dataBtns.length,
      dataBtnTexts: dataBtns.map(b => b.textContent.trim()),
      importBtnCount: importBtns.length,
      importBtnTexts: importBtns.map(b => b.textContent.trim()),
      swatchCount: swatches.length,
      memberCount: memberChips.length,
      setLabelTags,
      overflowCount: overflowEls.length,
      ariaModal,
      roleDialog,
      hasSaveBtn: !!saveBtn,
      hasCancelBtn: !!cancelBtn,
      hasCloseBtn: !!closeBtn,
      footPinned,
      // computed colors
      backdropBg,
      faceBg,
      toggleSpanColor,
      toggleSpanBg,
      setLabelColor,
    };
  }, tag);
}

async function run() {
  const browser = await chromium.launch({ headless: true });
  const errors = [];

  // ─── DARK 1280 ───────────────────────────────────────────────────────────────
  console.log('[GM1] dark 1280...');
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    page.on('pageerror', e => errors.push(`dark-1280: ${e.message}`));
    await bootDark(page);
    await openSettings(page);

    await page.screenshot({ path: ssPath('gm01_dark_1280_top.png'), fullPage: false });
    // scroll mid
    await page.evaluate(() => { const b = document.querySelector('.set-body'); if (b) b.scrollTop = b.scrollHeight / 2; });
    await page.waitForTimeout(200);
    await page.screenshot({ path: ssPath('gm01_dark_1280_mid.png'), fullPage: false });
    // scroll bottom
    await page.evaluate(() => { const b = document.querySelector('.set-body'); if (b) b.scrollTop = b.scrollHeight; });
    await page.waitForTimeout(200);
    await page.screenshot({ path: ssPath('gm01_dark_1280_bottom.png'), fullPage: false });
    // scroll back to top for audit
    await page.evaluate(() => { const b = document.querySelector('.set-body'); if (b) b.scrollTop = 0; });

    const domDark = await auditModal(page, 'dark-1280');
    fs.writeFileSync(dataPath('gm01_dark_1280_dom.json'), JSON.stringify(domDark, null, 2));
    console.log('  sectionCount:', domDark.sectionCount, '  inputCount:', domDark.inputCount, '  toggleRowCount:', domDark.toggleRowCount);
    console.log('  backdropBg:', domDark.backdropBg, '  faceBg:', domDark.faceBg);
    console.log('  toggleSpanColor:', domDark.toggleSpanColor, '  setLabelColor:', domDark.setLabelColor);
    console.log('  importBtns:', domDark.importBtnTexts);
    console.log('  overflowCount:', domDark.overflowCount);
    console.log('  flipWrapDims:', domDark.flipWrapDims);
    console.log('  setLabelTags:', domDark.setLabelTags);
    console.log('  ariaModal:', domDark.ariaModal, '  roleDialog:', domDark.roleDialog);
    console.log('  footPinned:', domDark.footPinned);

    await closeSettings(page);
    await ctx.close();
  }

  // ─── DARK 768 ────────────────────────────────────────────────────────────────
  console.log('[GM1] dark 768...');
  {
    const ctx = await browser.newContext({ viewport: { width: 768, height: 900 } });
    const page = await ctx.newPage();
    page.on('pageerror', e => errors.push(`dark-768: ${e.message}`));
    await bootDark(page);
    await openSettings(page);
    await page.screenshot({ path: ssPath('gm01_dark_768.png'), fullPage: false });

    const dom768 = await auditModal(page, 'dark-768');
    fs.writeFileSync(dataPath('gm01_dark_768_dom.json'), JSON.stringify(dom768, null, 2));
    console.log('  overflowCount at 768:', dom768.overflowCount, '  flipWrapDims:', dom768.flipWrapDims);

    await closeSettings(page);
    await ctx.close();
  }

  // ─── LIGHT 1280 ──────────────────────────────────────────────────────────────
  console.log('[GM1] light 1280...');
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    page.on('pageerror', e => errors.push(`light-1280: ${e.message}`));
    await bootLight(page);
    const themeAttr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    console.log('  data-theme:', themeAttr);
    await openSettings(page);

    await page.screenshot({ path: ssPath('gm01_light_1280_top.png'), fullPage: false });
    await page.evaluate(() => { const b = document.querySelector('.set-body'); if (b) b.scrollTop = b.scrollHeight / 2; });
    await page.waitForTimeout(200);
    await page.screenshot({ path: ssPath('gm01_light_1280_mid.png'), fullPage: false });
    await page.evaluate(() => { const b = document.querySelector('.set-body'); if (b) b.scrollTop = b.scrollHeight; });
    await page.waitForTimeout(200);
    await page.screenshot({ path: ssPath('gm01_light_1280_bottom.png'), fullPage: false });
    await page.evaluate(() => { const b = document.querySelector('.set-body'); if (b) b.scrollTop = 0; });

    const domLight = await auditModal(page, 'light-1280');
    fs.writeFileSync(dataPath('gm01_light_1280_dom.json'), JSON.stringify(domLight, null, 2));
    console.log('  backdropBg:', domLight.backdropBg, '  faceBg:', domLight.faceBg);
    console.log('  toggleSpanColor:', domLight.toggleSpanColor);
    console.log('  setLabelColor:', domLight.setLabelColor);
    console.log('  overflowCount:', domLight.overflowCount);

    await closeSettings(page);
    await ctx.close();
  }

  // ─── LIGHT 768 ───────────────────────────────────────────────────────────────
  console.log('[GM1] light 768...');
  {
    const ctx = await browser.newContext({ viewport: { width: 768, height: 900 } });
    const page = await ctx.newPage();
    page.on('pageerror', e => errors.push(`light-768: ${e.message}`));
    await bootLight(page);
    await openSettings(page);
    await page.screenshot({ path: ssPath('gm01_light_768.png'), fullPage: false });

    const domLight768 = await auditModal(page, 'light-768');
    fs.writeFileSync(dataPath('gm01_light_768_dom.json'), JSON.stringify(domLight768, null, 2));
    console.log('  overflowCount at 768:', domLight768.overflowCount, '  flipWrapDims:', domLight768.flipWrapDims);
    console.log('  toggleSpanColor:', domLight768.toggleSpanColor, '  backdropBg:', domLight768.backdropBg);

    await closeSettings(page);
    await ctx.close();
  }

  await browser.close();

  if (errors.length > 0) {
    console.error('[GM1] PAGE ERRORS:', errors);
    process.exit(1);
  }

  console.log('[GM1] Done. Screenshots written to e2e/screenshots/gm01_*.png');
  console.log('[GM1] JSON data written to e2e/screenshots/gm01_*.json');
  process.exit(0);
}

run().catch(e => { console.error(e); process.exit(1); });
