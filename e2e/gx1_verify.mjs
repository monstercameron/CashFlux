import { chromium } from 'playwright';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const BASE_URL = 'http://localhost:8080';

function parseRGB(str) {
  // handles both rgb(...) and rgba(...)
  const m = str.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
  if (!m) return null;
  return { r: parseInt(m[1]), g: parseInt(m[2]), b: parseInt(m[3]) };
}

function isLight(rgb) {
  if (!rgb) return false;
  return rgb.r > 200 && rgb.g > 200 && rgb.b > 200;
}

function isDark(rgb) {
  if (!rgb) return false;
  return rgb.r < 80 && rgb.g < 80 && rgb.b < 80;
}

async function removeErrorOverlay(page) {
  await page.evaluate(() => {
    const el = document.querySelector('gwc-error-overlay');
    if (el) el.remove();
  });
}

async function measureBg(page, selector) {
  try {
    return await page.evaluate((sel) => {
      const el = document.querySelector(sel);
      if (!el) return null;
      const s = getComputedStyle(el);
      return { bg: s.backgroundColor, color: s.color };
    }, selector);
  } catch {
    return null;
  }
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 800 });

  // ── LIGHT MODE SETUP ──────────────────────────────────────────────────────
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' })));
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 10000 });
  await removeErrorOverlay(page);

  // Give UI time to settle
  await page.waitForTimeout(1000);
  await removeErrorOverlay(page);

  // ── MEASURE LIGHT MODE ────────────────────────────────────────────────────
  const topbarLight = await measureBg(page, '.topbar');
  const railLight   = await measureBg(page, 'aside.rail');

  // Try .nv.active first, fall back to [aria-current="page"]
  let nvLight = await measureBg(page, '.nv.active');
  let nvSelector = '.nv.active';
  if (!nvLight) {
    nvLight = await measureBg(page, '.nv[aria-current="page"]');
    nvSelector = '.nv[aria-current="page"]';
  }

  const addBtnLight = await measureBg(page, '.add-btn');

  // ── SHELL SCREENSHOT (light, before menu) ─────────────────────────────────
  const shellLightPath = path.join(__dirname, 'gx1_verify_shell_light.png');
  await page.screenshot({ path: shellLightPath, fullPage: false });
  console.log(`Screenshot saved: ${shellLightPath}`);

  // ── OPEN +ADD MENU ────────────────────────────────────────────────────────
  let addMenuLight = null;
  try {
    await page.click('.add-btn');
    await page.waitForTimeout(600);
    await removeErrorOverlay(page);
    addMenuLight = await measureBg(page, '.add-menu');
  } catch (e) {
    console.log('  [WARN] Could not open .add-btn or measure .add-menu:', e.message);
  }

  // ── ADD MENU SCREENSHOT ───────────────────────────────────────────────────
  const addMenuLightPath = path.join(__dirname, 'gx1_verify_addmenu_light.png');
  await page.screenshot({ path: addMenuLightPath, fullPage: false });
  console.log(`Screenshot saved: ${addMenuLightPath}`);

  // ── DARK MODE ─────────────────────────────────────────────────────────────
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'dark' })));
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'dark', { timeout: 10000 });
  await removeErrorOverlay(page);
  await page.waitForTimeout(800);
  await removeErrorOverlay(page);

  const topbarDark = await measureBg(page, '.topbar');

  const shellDarkPath = path.join(__dirname, 'gx1_verify_shell_dark.png');
  await page.screenshot({ path: shellDarkPath, fullPage: false });
  console.log(`Screenshot saved: ${shellDarkPath}`);

  await browser.close();

  // ── REPORT ────────────────────────────────────────────────────────────────
  console.log('\n════════════════════════════════════════════════════');
  console.log('GX-1 LIGHT MODE CSS VERIFICATION');
  console.log('════════════════════════════════════════════════════');

  function report(label, result, passTest, extraNote) {
    const bg  = result ? result.bg    : 'NOT FOUND';
    const col = result ? result.color : 'NOT FOUND';
    const rgb = result ? parseRGB(result.bg) : null;
    const pass = result && passTest(rgb);
    console.log(`\n  ${label}`);
    console.log(`    backgroundColor : ${bg}`);
    console.log(`    color           : ${col}`);
    console.log(`    RESULT          : ${pass ? 'PASS' : 'FAIL'}${extraNote ? '  ' + extraNote : ''}`);
  }

  report('.topbar (light)',           topbarLight,   isLight);
  report('aside.rail (light)',        railLight,     isLight);
  report(`${nvSelector} (light)`,     nvLight,       (rgb) => rgb && (isLight(rgb) || (rgb.r > 100)));
  report('.add-btn (light)',          addBtnLight,   isLight);
  report('.add-menu (light)',         addMenuLight,  isLight);

  console.log('\n════════════════════════════════════════════════════');
  console.log('DARK REGRESSION CHECK');
  console.log('════════════════════════════════════════════════════');
  report('.topbar (dark)',            topbarDark,    isDark);

  console.log('\n════════════════════════════════════════════════════');
  console.log('Screenshots:');
  console.log('  ' + shellLightPath);
  console.log('  ' + addMenuLightPath);
  console.log('  ' + shellDarkPath);
  console.log('════════════════════════════════════════════════════\n');

  // Exit code: 0 if all pass, 1 if any fail
  const allPass = [
    topbarLight   && isLight(parseRGB(topbarLight.bg)),
    railLight     && isLight(parseRGB(railLight.bg)),
    nvLight       && (isLight(parseRGB(nvLight.bg)) || (parseRGB(nvLight.bg) && parseRGB(nvLight.bg).r > 100)),
    addBtnLight   && isLight(parseRGB(addBtnLight.bg)),
    addMenuLight  && isLight(parseRGB(addMenuLight.bg)),
    topbarDark    && isDark(parseRGB(topbarDark.bg)),
  ];

  process.exit(allPass.every(Boolean) ? 0 : 1);
})();
