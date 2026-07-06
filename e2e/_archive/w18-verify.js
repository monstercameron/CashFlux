const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const screenshotsDir = path.join(__dirname, 'screenshots');

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();

  // Collect console errors
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  console.log('Navigating to dashboard...');
  await page.goto('http://127.0.0.1:8099/#/dashboard');

  // Wait ~1 second for WASM to load
  await page.waitForTimeout(2500);

  // Screenshot 1: settled state
  await page.screenshot({ path: path.join(screenshotsDir, 'w18-chart-settled.png'), fullPage: false });
  console.log('Saved w18-chart-settled.png');

  // Check for wonder-chart-line path element
  const lineInfo = await page.evaluate(() => {
    const paths = document.querySelectorAll('path.wonder-chart-line');
    if (paths.length === 0) return { found: false };
    const el = paths[0];
    const pathLength = el.getAttribute('pathLength');
    const computedAnimName = getComputedStyle(el).animationName;
    return {
      found: true,
      count: paths.length,
      pathLength,
      animationName: computedAnimName,
      outerHTML: el.outerHTML.substring(0, 300),
    };
  });
  console.log('wonder-chart-line (default state):', JSON.stringify(lineInfo, null, 2));

  // Set data-wonder="off"
  await page.evaluate(() => {
    document.documentElement.setAttribute('data-wonder', 'off');
  });
  await page.waitForTimeout(300);

  // Screenshot 2: wonder off
  await page.screenshot({ path: path.join(screenshotsDir, 'w18-chart-wonder-off.png'), fullPage: false });
  console.log('Saved w18-chart-wonder-off.png');

  // Check animation-name with wonder=off
  const lineInfoOff = await page.evaluate(() => {
    const paths = document.querySelectorAll('path.wonder-chart-line');
    if (paths.length === 0) return { found: false };
    const el = paths[0];
    return {
      animationName: getComputedStyle(el).animationName,
    };
  });
  console.log('wonder-chart-line (wonder=off):', JSON.stringify(lineInfoOff, null, 2));

  // Remove data-wonder attribute (restore default)
  await page.evaluate(() => {
    document.documentElement.removeAttribute('data-wonder');
  });
  await page.waitForTimeout(200);

  const lineInfoRestored = await page.evaluate(() => {
    const paths = document.querySelectorAll('path.wonder-chart-line');
    if (paths.length === 0) return { found: false };
    const el = paths[0];
    return {
      animationName: getComputedStyle(el).animationName,
    };
  });
  console.log('wonder-chart-line (attribute removed/restored):', JSON.stringify(lineInfoRestored, null, 2));

  // Summary
  console.log('\n--- SUMMARY ---');
  console.log('wonder-chart-line found:', lineInfo.found);
  console.log('count:', lineInfo.count);
  console.log('pathLength attribute:', lineInfo.pathLength);
  console.log('animationName (default):', lineInfo.animationName);
  console.log('animationName (wonder=off):', lineInfoOff.animationName);
  console.log('animationName (restored):', lineInfoRestored.animationName);
  console.log('Console errors:', consoleErrors.length > 0 ? consoleErrors : 'none');

  await browser.close();
})();
