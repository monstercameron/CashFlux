import { chromium } from 'playwright';

const BASE = process.env.BASE_URL || 'http://localhost:7717';

async function measureElements(page, label) {
  const results = await page.evaluate(() => {
    const out = {};

    // 1) .w tiles
    const wTiles = [...document.querySelectorAll('.w')].slice(0, 4);
    out.wTiles = wTiles.map((el, i) => ({
      index: i,
      bg: getComputedStyle(el).backgroundColor,
      class: el.className.substring(0, 80),
    }));

    // 2) .wh-title elements
    const titles = [...document.querySelectorAll('.wh .wh-title, .wh h2, .wh h3')].slice(0, 4);
    out.titles = titles.map((el, i) => ({
      index: i,
      color: getComputedStyle(el).color,
      tag: el.tagName,
      class: el.className.substring(0, 80),
      text: el.textContent.trim().substring(0, 40),
    }));

    // 3) .bento > * wrappers
    const bentoChildren = [...document.querySelectorAll('.bento > *')].slice(0, 4);
    out.bentoChildren = bentoChildren.map((el, i) => ({
      index: i,
      bg: getComputedStyle(el).backgroundColor,
      class: el.className.substring(0, 80),
    }));

    return out;
  });
  console.log(`\n=== ${label} ===`);
  console.log('--- .w tiles (EXPECT white rgb(255,255,255) in light) ---');
  results.wTiles.forEach(t => console.log(`  [${t.index}] bg=${t.bg}  class="${t.class}"`));
  console.log('--- .wh .wh-title / h2 / h3 (EXPECT dark ~rgb(28,28,30) in light) ---');
  results.titles.forEach(t => console.log(`  [${t.index}] color=${t.color}  tag=${t.tag}  text="${t.text}"`));
  console.log('--- .bento > * wrappers (confirm transparent = false-alarm) ---');
  results.bentoChildren.forEach(t => console.log(`  [${t.index}] bg=${t.bg}  class="${t.class}"`));
  return results;
}

function isWhite(bg) {
  return bg === 'rgb(255, 255, 255)' || bg === 'rgba(255, 255, 255, 1)';
}
function isTransparent(bg) {
  return bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent';
}
function isDark(bg) {
  // rgb(18,18,20) or similar very dark
  const m = bg.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
  if (!m) return false;
  return parseInt(m[1]) < 40 && parseInt(m[2]) < 40 && parseInt(m[3]) < 40;
}
function isNearWhiteColor(color) {
  const m = color.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
  if (!m) return false;
  return parseInt(m[1]) > 200 && parseInt(m[2]) > 200 && parseInt(m[3]) > 200;
}
function isDarkColor(color) {
  const m = color.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
  if (!m) return false;
  return parseInt(m[1]) < 80 && parseInt(m[2]) < 80 && parseInt(m[3]) < 80;
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();

  // --- LIGHT MODE ---
  await page.goto(BASE + '/');  // dashboard is served at root; /dashboard is a 404 on the gwc static server
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' })));
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 10000 });
  await page.waitForTimeout(500);

  const lightResults = await measureElements(page, 'LIGHT MODE');
  await page.screenshot({ path: 'e2e/gx11_verify_dashboard_light.png', fullPage: false });
  console.log('\nScreenshot saved: e2e/gx11_verify_dashboard_light.png');

  // --- PASS/FAIL for light ---
  console.log('\n=== LIGHT MODE VERDICTS ===');
  let lightPass = true;
  lightResults.wTiles.forEach((t, i) => {
    const pass = isWhite(t.bg);
    const fail = isTransparent(t.bg) || isDark(t.bg);
    console.log(`  .w[${i}] bg=${t.bg} => ${pass ? 'PASS (white)' : fail ? 'FAIL' : 'WARN (unexpected)'}`);
    if (!pass) lightPass = false;
  });
  lightResults.titles.forEach((t, i) => {
    const pass = isDarkColor(t.color);
    const fail = isNearWhiteColor(t.color);
    console.log(`  title[${i}] color=${t.color} => ${pass ? 'PASS (dark)' : fail ? 'FAIL (near-white)' : 'WARN'}`);
    if (!pass) lightPass = false;
  });
  console.log(`\nLIGHT MODE OVERALL: ${lightPass ? 'PASS' : 'FAIL'}`);

  // --- DARK MODE ---
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'dark' })));
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'dark', { timeout: 10000 });
  await page.waitForTimeout(500);

  const darkResults = await measureElements(page, 'DARK MODE');
  await page.screenshot({ path: 'e2e/gx11_verify_dashboard_dark.png', fullPage: false });
  console.log('\nScreenshot saved: e2e/gx11_verify_dashboard_dark.png');

  console.log('\n=== DARK MODE VERDICTS (regression check) ===');
  let darkPass = true;
  darkResults.wTiles.forEach((t, i) => {
    const pass = isDark(t.bg) || !isWhite(t.bg); // tiles should NOT be white in dark mode
    console.log(`  .w[${i}] bg=${t.bg} => ${isDark(t.bg) ? 'PASS (dark)' : isWhite(t.bg) ? 'FAIL (white in dark mode)' : 'OK (non-white)'}`);
  });
  darkResults.titles.forEach((t, i) => {
    const pass = isNearWhiteColor(t.color);
    console.log(`  title[${i}] color=${t.color} => ${pass ? 'PASS (light text)' : 'FAIL (dark text in dark mode)'}`);
    if (!pass) darkPass = false;
  });
  console.log(`\nDARK MODE OVERALL: ${darkPass ? 'PASS' : 'FAIL'}`);

  await browser.close();
})();
