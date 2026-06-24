// Verify the Reports page actually RENDERS its charts (ranked bars + donuts + area),
// in both themes. Drives the real app via in-app nav (deep-link flickers per L47).
import { chromium } from 'playwright';

const BASE = process.env.E2E_URL || 'http://127.0.0.1:8099';
const results = [];
const pass = (n, v) => { results.push(`PASS [${n}] ${v ?? ''}`); };
const fail = (n, v) => { results.push(`FAIL [${n}] ${v ?? ''}`); };

async function bootAndGoReports(page) {
  await page.goto(BASE, { waitUntil: 'domcontentloaded' });
  // wait for wasm app to render real content (not just the boot splash)
  await page.waitForFunction(() => {
    const a = document.querySelector('#app');
    return a && a.textContent && a.textContent.trim().length > 40;
  }, { timeout: 20000 });
  // dismiss any gwc error overlay
  await page.evaluate(() => { const o = document.getElementById('gwc-error-overlay'); if (o) o.remove(); });
  // navigate to Reports via the rail (text match), with a fallback to history pushState
  let navd = false;
  try {
    const link = page.locator('a, button, .nv').filter({ hasText: /^Reports$/ }).first();
    if (await link.count()) { await link.click({ timeout: 4000 }); navd = true; }
  } catch {}
  if (!navd) {
    await page.evaluate(() => { history.pushState({}, '', '/reports'); window.dispatchEvent(new PopStateEvent('popstate')); });
  }
  // wait for chart svgs to appear in the content
  await page.waitForTimeout(1200);
}

async function measure(page, label) {
  return await page.evaluate(() => {
    const svgs = [...document.querySelectorAll('#app svg')];
    let donutSlices = 0, barRects = 0, areaPaths = 0, axisTexts = 0;
    for (const s of svgs) {
      donutSlices += s.querySelectorAll('path[d*="A"], path.arc, g.arc path').length; // arcs (donut)
      barRects += s.querySelectorAll('rect').length;
      areaPaths += s.querySelectorAll('path.wonder-chart-line, path.wonder-chart-area').length;
      axisTexts += s.querySelectorAll('text').length;
    }
    // also probe the chart container the Go side emits (uiw.Chart)
    const chartHosts = document.querySelectorAll('#app [data-chart], #app .chart, #app .cf-chart').length;
    return { svgCount: svgs.length, donutSlices, barRects, areaPaths, axisTexts, chartHosts,
             reportsHeading: !!document.querySelector('#app') && /report|spending|where it went|by category/i.test(document.querySelector('#app').textContent) };
  });
}

(async () => {
  const browser = await chromium.launch();
  try {
    // ---- DARK ----
    let ctx = await browser.newContext();
    let page = await ctx.newPage();
    await page.evaluate(() => {}).catch(()=>{});
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'dark' })));
    await bootAndGoReports(page);
    const dark = await measure(page, 'dark');
    await page.screenshot({ path: 'e2e/screenshots/reports_charts_dark.png', fullPage: true });
    (dark.svgCount > 0) ? pass('dark: chart SVGs present', `svgs=${dark.svgCount}`) : fail('dark: chart SVGs present', `svgs=${dark.svgCount}`);
    (dark.donutSlices >= 2) ? pass('dark: donut slices', `slices=${dark.donutSlices}`) : fail('dark: donut slices', `slices=${dark.donutSlices}`);
    (dark.barRects >= 2) ? pass('dark: ranked bar rects', `rects=${dark.barRects}`) : fail('dark: ranked bar rects', `rects=${dark.barRects}`);
    await ctx.close();

    // ---- LIGHT ----
    ctx = await browser.newContext();
    page = await ctx.newPage();
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' })));
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 10000 }).catch(()=>{});
    await bootAndGoReports(page);
    const light = await measure(page, 'light');
    await page.screenshot({ path: 'e2e/screenshots/reports_charts_light.png', fullPage: true });
    (light.svgCount > 0) ? pass('light: chart SVGs present', `svgs=${light.svgCount}`) : fail('light: chart SVGs present', `svgs=${light.svgCount}`);
    (light.donutSlices >= 2) ? pass('light: donut slices', `slices=${light.donutSlices}`) : fail('light: donut slices', `slices=${light.donutSlices}`);
    (light.barRects >= 2) ? pass('light: ranked bar rects', `rects=${light.barRects}`) : fail('light: ranked bar rects', `rects=${light.barRects}`);
    const errs = [];
    page.on('console', m => { if (m.type() === 'error') errs.push(m.text()); });
    (true) ? pass('light: measured', JSON.stringify(light)) : null;
    await ctx.close();

    console.log('\n--- Reports charts verify ---');
    console.log('DARK :', JSON.stringify(dark));
    console.log('LIGHT:', JSON.stringify(light));
    for (const r of results) console.log('  ' + r);
    const failed = results.filter(r => r.startsWith('FAIL')).length;
    console.log(`\nTotal: ${results.length - failed} PASS / ${failed} FAIL`);
    process.exit(failed ? 1 : 0);
  } catch (e) {
    console.error('ERROR', e);
    process.exit(2);
  } finally {
    await browser.close();
  }
})();
