const { chromium } = require('playwright');
const path = require('path');

const SCREENSHOTS = path.join(__dirname, 'screenshots');
const BASE = 'http://127.0.0.1:8099';

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

(async () => {
  const browser = await chromium.launch({ headless: true });

  // ---- PART A: verify wonder_visible timepoints ----
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    const errors = [];
    page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });

    console.log('=== STEP 3: VERIFY AMPLIFIED ANIMATIONS ===');
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(1000); // WASM boot

    // Re-confirm state
    const dataWonder = await page.evaluate(() => document.documentElement.getAttribute('data-wonder'));
    const wonderOn = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--wonder-on').trim());
    const wonderLift = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--wonder-lift').trim());
    const reducedMotion = await page.evaluate(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    console.log('data-wonder:', dataWonder, '| --wonder-on:', wonderOn, '| --wonder-lift:', wonderLift);
    console.log('reduced-motion:', reducedMotion);

    // Find Transactions link and click it, then capture at precise intervals
    const txnLink = await page.$('button:has-text("Transactions"), a:has-text("Transactions"), .nav-link:has-text("Transactions"), .nv:has-text("Transactions")');
    if (txnLink) {
      // Take t=0 immediately after click (no wait)
      await txnLink.click();

      const delayMap = {
        't0': 0,
        't60': 60,
        't120': 120,
        't200': 200,
        't320': 320,
      };

      let started = Date.now();
      for (const [label, target] of Object.entries(delayMap)) {
        const elapsed = Date.now() - started;
        const remaining = target - elapsed;
        if (remaining > 0) await sleep(remaining);

        const ssPath = path.join(SCREENSHOTS, `wonder_visible_${label}.png`);
        await page.screenshot({ path: ssPath, fullPage: false });

        const metrics = await page.evaluate(() => {
          const el = document.getElementById('cf-page-view');
          if (!el) return { error: 'no #cf-page-view' };
          const s = getComputedStyle(el);
          return {
            opacity: s.opacity,
            transform: s.transform,
            animationName: s.animationName,
            animationDuration: s.animationDuration,
            animationPlayState: s.animationPlayState,
            hasPageEnter: el.classList.contains('page-enter'),
          };
        });
        console.log(`${label} (actual +${Date.now()-started}ms):`, JSON.stringify(metrics));
        console.log(`  -> screenshot: ${ssPath}`);
      }
    } else {
      console.log('WARNING: Could not find Transactions nav link');
      // dump nav items for debug
      const navItems = await page.evaluate(() => {
        return [...document.querySelectorAll('.nv, .nav-link')].map(el => ({
          tag: el.tagName,
          text: el.textContent.trim().slice(0, 30),
          cls: el.className,
        }));
      });
      console.log('Nav items found:', JSON.stringify(navItems, null, 2));
    }

    // ---- CARD / TILE HOVER ----
    console.log('\n=== CARD/TILE HOVER VERIFY ===');
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(800);

    // Try .card first, then .w tile
    let hoverTarget = await page.$('.card');
    let targetType = '.card';
    if (!hoverTarget) {
      hoverTarget = await page.$('.w:not(.drag)');
      targetType = '.w';
    }

    if (hoverTarget) {
      // Before hover
      const beforePath = path.join(SCREENSHOTS, 'wonder_card_hover_before.png');
      await page.screenshot({ path: beforePath });
      const beforeMetrics = await hoverTarget.evaluate(el => {
        const s = getComputedStyle(el);
        return { transform: s.transform, boxShadow: s.boxShadow };
      });
      console.log(`${targetType} BEFORE hover:`, JSON.stringify(beforeMetrics));
      console.log('Before screenshot:', beforePath);

      // Hover
      await hoverTarget.hover();
      await sleep(400); // let transition settle (170ms dur)

      const afterPath = path.join(SCREENSHOTS, 'wonder_card_hover_after.png');
      await page.screenshot({ path: afterPath });
      const afterMetrics = await hoverTarget.evaluate(el => {
        const s = getComputedStyle(el);
        return { transform: s.transform, boxShadow: s.boxShadow };
      });
      console.log(`${targetType} AFTER hover:`, JSON.stringify(afterMetrics));
      console.log('After screenshot:', afterPath);
    } else {
      console.log('WARNING: No hoverable element found');
    }

    console.log('\nConsole errors:', errors.length ? errors : 'none');
    await ctx.close();
  }

  // ---- PART B: verify data-wonder="off" → fully static ----
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    console.log('\n=== WONDER=OFF STATIC CHECK ===');
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(800);

    // Force data-wonder=off
    await page.evaluate(() => document.documentElement.setAttribute('data-wonder', 'off'));
    await sleep(100);

    const offWonderOn = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--wonder-on').trim());
    const offLift = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--wonder-lift').trim());
    console.log('[data-wonder=off] --wonder-on:', offWonderOn, '| --wonder-lift:', offLift);

    // Navigate and check no animation
    const txnLink = await page.$('button:has-text("Transactions"), .nav-link:has-text("Transactions"), .nv:has-text("Transactions")');
    if (txnLink) {
      await txnLink.click();
      await sleep(50);
      const metrics = await page.evaluate(() => {
        const el = document.getElementById('cf-page-view');
        if (!el) return { error: 'no #cf-page-view' };
        const s = getComputedStyle(el);
        return {
          opacity: s.opacity,
          transform: s.transform,
          animationName: s.animationName,
          hasPageEnter: el.classList.contains('page-enter'),
        };
      });
      console.log('wonder=off after nav at t=50ms:', JSON.stringify(metrics));
      console.log('(opacity should be 1, animation should be "none")');
    }
    await ctx.close();
  }

  // ---- PART C: simulate reduced-motion ----
  {
    const ctx = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      reducedMotion: 'reduce',
    });
    const page = await ctx.newPage();
    console.log('\n=== REDUCED-MOTION STATIC CHECK ===');
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(800);

    const rm = await page.evaluate(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    console.log('reduced-motion emulated:', rm);

    const txnLink = await page.$('button:has-text("Transactions"), .nav-link:has-text("Transactions"), .nv:has-text("Transactions")');
    if (txnLink) {
      await txnLink.click();
      await sleep(50);
      const metrics = await page.evaluate(() => {
        const el = document.getElementById('cf-page-view');
        if (!el) return { error: 'no #cf-page-view' };
        const s = getComputedStyle(el);
        return { opacity: s.opacity, transform: s.transform, animationName: s.animationName };
      });
      console.log('reduced-motion after nav at t=50ms:', JSON.stringify(metrics));
    }
    await ctx.close();
  }

  await browser.close();
  console.log('\n=== DONE ===');
})();
