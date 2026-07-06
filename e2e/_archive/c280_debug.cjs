'use strict';
const { chromium } = require('C:\\Users\\mreca\\Desktop\\CashFlux\\.tools\\node_modules\\playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error' && !/already exited/i.test(msg.text())) errors.push(msg.text());
  });

  await page.goto('http://127.0.0.1:8099/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);

  const body1 = await page.evaluate(() => document.body.innerText.substring(0, 500));
  console.log('Home page (first 500):', body1);

  // Check for sample-start-fresh
  const sf = await page.$('[data-testid="sample-start-fresh"]');
  console.log('sample-start-fresh present:', !!sf);

  if (sf) {
    await sf.click();
    await page.waitForTimeout(1500);
  }

  // Navigate to members
  await page.goto('http://127.0.0.1:8099/members', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);

  const body2 = await page.evaluate(() => document.body.innerText);
  console.log('Members page text:', body2.substring(0, 1000));

  await page.screenshot({ path: 'e2e/screenshots/c280_debug.png', fullPage: true });
  await browser.close();
})();
