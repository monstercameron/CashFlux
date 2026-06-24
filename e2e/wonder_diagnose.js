const { chromium } = require('playwright');
const path = require('path');

const SCREENSHOTS = path.join(__dirname, 'screenshots');
const BASE = 'http://127.0.0.1:8099';

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: 1280, height: 800 },
  });
  const page = await ctx.newPage();

  // Collect console errors
  const errors = [];
  page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });

  console.log('=== STEP 1: DIAGNOSE ===');
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await sleep(800); // let WASM boot

  const dataWonder = await page.evaluate(() => document.documentElement.getAttribute('data-wonder'));
  const wonderOn = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--wonder-on').trim());
  const reducedMotion = await page.evaluate(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  const motionNoPreference = await page.evaluate(() => window.matchMedia('(prefers-reduced-motion: no-preference)').matches);

  console.log('data-wonder:', JSON.stringify(dataWonder));
  console.log('--wonder-on:', JSON.stringify(wonderOn));
  console.log('prefers-reduced-motion: reduce matches:', reducedMotion);
  console.log('prefers-reduced-motion: no-preference matches:', motionNoPreference);

  // Check what data-wonder the app actually sets
  const htmlAttrs = await page.evaluate(() => {
    const el = document.documentElement;
    return {
      dataWonder: el.getAttribute('data-wonder'),
      dataTheme: el.getAttribute('data-theme'),
      dataDensity: el.getAttribute('data-density'),
    };
  });
  console.log('HTML attrs:', JSON.stringify(htmlAttrs));

  // Navigate Dashboard → Transactions and capture timepoints
  console.log('\n=== PAGE ENTER ANIMATION CAPTURE (before amplification) ===');
  // Find Transactions nav link
  const txnLink = await page.$('button:has-text("Transactions"), a:has-text("Transactions"), .nav-link:has-text("Transactions")');
  if (txnLink) {
    // Measure page-view element before clicking
    const t0Screenshot = path.join(SCREENSHOTS, 'diag_before_nav.png');
    await page.screenshot({ path: t0Screenshot });
    console.log('Before nav screenshot:', t0Screenshot);

    await txnLink.click();
    // Capture at timepoints
    const delays = [0, 60, 120, 200, 320];
    for (const delay of delays) {
      await sleep(delay);
      const ss = path.join(SCREENSHOTS, `diag_t${delay}.png`);
      await page.screenshot({ path: ss });

      // Measure transform/opacity on #cf-page-view
      const metrics = await page.evaluate(() => {
        const el = document.getElementById('cf-page-view');
        if (!el) return { error: 'no #cf-page-view' };
        const s = getComputedStyle(el);
        return {
          opacity: s.opacity,
          transform: s.transform,
          animationName: s.animationName,
          animationDuration: s.animationDuration,
          hasPageEnter: el.classList.contains('page-enter'),
        };
      });
      console.log(`t=${delay}ms:`, JSON.stringify(metrics));
    }
  } else {
    console.log('WARNING: Transactions nav link not found');
    await page.screenshot({ path: path.join(SCREENSHOTS, 'diag_nav_missing.png') });
  }

  // Card hover capture
  console.log('\n=== CARD HOVER (before amplification) ===');
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await sleep(800);
  const card = await page.$('.card');
  if (card) {
    await page.screenshot({ path: path.join(SCREENSHOTS, 'diag_card_before.png') });
    const beforeMetrics = await card.evaluate(el => {
      const s = getComputedStyle(el);
      return { transform: s.transform, boxShadow: s.boxShadow };
    });
    console.log('Card before hover:', JSON.stringify(beforeMetrics));

    await card.hover();
    await sleep(400);
    await page.screenshot({ path: path.join(SCREENSHOTS, 'diag_card_after.png') });
    const afterMetrics = await card.evaluate(el => {
      const s = getComputedStyle(el);
      return { transform: s.transform, boxShadow: s.boxShadow };
    });
    console.log('Card after hover:', JSON.stringify(afterMetrics));
  } else {
    console.log('WARNING: No .card found on dashboard');
  }

  console.log('\nConsole errors:', errors.length ? errors : 'none');
  await browser.close();
})();
