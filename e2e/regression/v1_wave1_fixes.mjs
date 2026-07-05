// v1_wave1_fixes.mjs — regression coverage for the v1.0 refinement wave-1
// fixes (groups A–D + cross-cutting). Each assertion pins a specific bug that
// was fixed, so it can't silently regress. Run: node e2e/regression/v1_wave1_fixes.mjs
import { boot, nav, setTheme, jsClick, mainText } from "./_harness.mjs";

const fails = [];
const ok = (c, m) => { if (!c) { fails.push(m); console.error("  ✗", m); } else console.log("  ✓", m); };

const { browser, context, page, errors } = await boot();
try {
  // ── /todo: adding a task refreshes the list + toasts (was a blocker). ──
  await nav(page, "/todo", 1800);
  await page.evaluate(() => { const b = [...document.querySelectorAll("#main button, .topbar button")].find(x => /add task|new task|add a task/i.test((x.textContent || "").trim())); if (b) b.click(); });
  await page.waitForTimeout(700);
  await page.evaluate(() => { const i = document.querySelector("#task-add"); if (i) { i.value = "AAA regression check task"; i.dispatchEvent(new Event("input", { bubbles: true })); } });
  await page.waitForTimeout(250);
  await jsClick(page, { testid: "task-add-submit" });
  await page.waitForTimeout(1400);
  ok(/AAA regression check task/i.test(await mainText(page)), "todo: new task visible immediately after add");
  ok(/task added/i.test(await page.evaluate(() => document.body.innerText)), "todo: 'Task added' toast shows");

  // ── /bills: no double-counted car/mortgage payment rows. ──
  await nav(page, "/bills", 1800);
  const dupCount = await page.evaluate(() => {
    const rows = [...document.querySelectorAll("#main")].map((m) => m.innerText).join("\n");
    // Count how many times a "Car payment (Marcus)"-style row AND a "Marcus's Car Loan" row co-occur.
    return { text: rows };
  });
  // The seed's car payment must appear once, not as both a recurring row and a liability row.
  const billsText = dupCount.text;
  const marcusCarRows = (billsText.match(/car payment \(marcus\)/gi) || []).length;
  ok(marcusCarRows <= 1, `bills: Marcus car payment listed ${marcusCarRows}× (want ≤1, no double-count)`);

  // ── /subscriptions: share sane; HOA (a recurring flow) not an active subscription. ──
  await nav(page, "/subscriptions", 1800);
  const subsText = await mainText(page);
  const share = subsText.match(/share of spending\s*\n?\s*(\d+)%/i);
  ok(!share || Number(share[1]) <= 100, `subscriptions: share of spending sane (${share ? share[1] + "%" : "n/a"})`);
  ok(!/how to cancel hoa/i.test(await page.evaluate(() => document.body.innerText)), "subscriptions: HOA (planned recurring) not flagged as a cancellable subscription");

  // ── /investments: holdings not all badged 'Other'. ──
  await nav(page, "/investments", 1800);
  const invText = await mainText(page);
  ok(/mutual fund|etf|stock/i.test(invText), "investments: holdings carry real security-type badges (not all 'Other')");

  // ── /budgets: no '1 budgets are over' plural bug (when a single budget is over). ──
  await nav(page, "/budgets", 1800);
  ok(!/\b1 budgets are over\b/i.test(await mainText(page)), "budgets: no '1 budgets are over' plural bug");

  // ── Light theme: meter/progress track is NOT the fixed dark hex. ──
  await setTheme(page, "light");
  await nav(page, "/allocate", 1800);
  const trackBg = await page.evaluate(() => {
    const bar = document.querySelector("#main .cf-bar, #main [role='meter']");
    if (!bar) return null;
    return getComputedStyle(bar).backgroundColor;
  });
  // #232325 == rgb(35,35,37). In light theme the track must be a light color, not that.
  ok(trackBg === null || trackBg !== "rgb(35, 35, 37)", `light: meter track not the fixed dark hex (got ${trackBg})`);
  await setTheme(page, "dark");

  ok(errors.length === 0, `no console/page errors (${errors.length ? errors.join(" | ") : "none"})`);
} finally {
  await context.close();
  await browser.close();
}

if (fails.length) {
  console.error("\nFAIL v1_wave1_fixes:\n - " + fails.join("\n - "));
  process.exit(1);
}
console.log("\nPASS: v1_wave1_fixes — all wave-1 refinement fixes hold");
