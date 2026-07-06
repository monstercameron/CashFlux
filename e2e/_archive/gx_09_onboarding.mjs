import { chromium } from 'playwright';

const BASE = 'http://localhost:8080';
const OUT = 'e2e/screenshots';
const THEMES = ['dark', 'light'];

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
  }, theme);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute('data-theme') === t,
    theme,
    { timeout: 5000 }
  ).catch(() => console.log(`  WARN: data-theme did not settle to ${theme}`));
}

async function clearFirstRunFlags(page) {
  await page.evaluate(() => {
    Object.keys(localStorage)
      .filter(k => k.startsWith('cashflux:'))
      .forEach(k => localStorage.removeItem(k));
  });
}

async function shot(page, name) {
  await page.screenshot({ path: `${OUT}/${name}`, fullPage: false });
  console.log(`  shot: ${name}`);
}

async function run() {
  const browser = await chromium.launch({ headless: true });

  for (const theme of THEMES) {
    console.log(`\n=== Theme: ${theme} ===`);
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 768 } });
    const page = await ctx.newPage();

    // --- BOOT SPLASH ---
    console.log('--- Boot splash capture ---');
    const navPromise = page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(300);
    await shot(page, `gx09_splash_early_${theme}_1280.png`);
    await page.waitForTimeout(700);
    await shot(page, `gx09_splash_mid_${theme}_1280.png`);
    await navPromise.catch(() => {});
    await page.waitForTimeout(500);
    await shot(page, `gx09_splash_late_${theme}_1280.png`);

    // Check if boot card lingers after app is interactive
    await page.waitForTimeout(3000);
    const bootState = await page.evaluate(() => {
      const boot = document.querySelector('#boot, .boot-card, [id*="boot"], [class*="boot"]');
      if (!boot) return { exists: false };
      const style = getComputedStyle(boot);
      return {
        exists: true,
        display: style.display,
        visibility: style.visibility,
        opacity: style.opacity,
        zIndex: style.zIndex,
        innerHTML: boot.innerHTML.substring(0, 200),
      };
    });
    console.log('  Boot element state after 3s:', JSON.stringify(bootState));

    const bootBg = await page.evaluate(() => {
      const boot = document.querySelector('#boot, .boot-card, [id*="boot"], [class*="boot"]');
      if (!boot) return null;
      return getComputedStyle(boot).backgroundColor;
    });
    console.log('  Boot card bg:', bootBg);

    await shot(page, `gx09_after_splash_${theme}_1280.png`);

    await setTheme(page, theme);
    await page.waitForTimeout(500);

    // --- FIRST-RUN / ONBOARDING ---
    console.log('--- First-run onboarding ---');
    await clearFirstRunFlags(page);
    await page.reload({ waitUntil: 'networkidle' }).catch(() => {});
    await page.waitForTimeout(2000);
    await shot(page, `gx09_firstrun_${theme}_1280.png`);

    const onboardingState = await page.evaluate(() => {
      const selectors = [
        '.onboarding', '#onboarding', '[class*="onboard"]',
        '.welcome', '#welcome', '[class*="welcome"]',
        '.tour', '#tour', '[class*="tour"]',
        '.guide', '[class*="guide"]',
        '.modal', '[class*="modal"]',
        '.overlay', '[class*="overlay"]',
        '.first-run', '[class*="first-run"]',
        '.setup', '[class*="setup"]',
      ];
      const found = [];
      selectors.forEach(sel => {
        try {
          const el = document.querySelector(sel);
          if (el) {
            const style = getComputedStyle(el);
            found.push({
              selector: sel,
              display: style.display,
              visibility: style.visibility,
              opacity: style.opacity,
              text: el.textContent.trim().substring(0, 100),
            });
          }
        } catch(e) {}
      });
      return {
        found,
        bodyText: document.body.textContent.trim().substring(0, 500),
        title: document.title,
      };
    });
    console.log('  Onboarding elements:', JSON.stringify(onboardingState, null, 2));

    // --- QUICK GUIDE ---
    console.log('--- Quick guide ---');
    const guideBtn = await page.evaluate(() => {
      const candidates = Array.from(document.querySelectorAll('button, a, [role="button"]'))
        .filter(el => /guide|tour|help|quick|start|begin/i.test(el.textContent));
      return candidates.map(el => ({
        text: el.textContent.trim(),
        class: el.className,
        visible: el.offsetParent !== null,
      }));
    });
    console.log('  Guide buttons:', JSON.stringify(guideBtn));

    try {
      const helpBtn = await page.$('button:has-text("guide"), button:has-text("Guide"), button:has-text("Quick"), [title*="guide"], [title*="Guide"], [aria-label*="guide"]');
      if (helpBtn) {
        await helpBtn.click();
        await page.waitForTimeout(500);
        await shot(page, `gx09_guide_${theme}_1280.png`);
      }
    } catch(e) {}

    // --- EMPTY STATE ---
    console.log('--- Empty dashboard state ---');
    await shot(page, `gx09_empty_dashboard_${theme}_1280.png`);

    const dashStyles = await page.evaluate(() => {
      const main = document.querySelector('main, .main, #main, [role="main"]');
      if (!main) return null;
      const style = getComputedStyle(main);
      return { bg: style.backgroundColor, color: style.color, padding: style.padding };
    });
    console.log('  Dashboard main styles:', dashStyles);

    const themeAttr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    console.log('  data-theme:', themeAttr);

    // Collect all localStorage keys
    const lsKeys = await page.evaluate(() => Object.keys(localStorage));
    console.log('  localStorage keys:', lsKeys);

    await ctx.close();
  }

  await browser.close();
  console.log('\n=== GX9 probe complete ===');
}

run().catch(e => { console.error(e); process.exit(1); });
