// v1_wave2_fixes — pins the second wave of v1.0 polish fixes (Groups I & J,
// custom pages + system settings). Run against a live `gwc dev` server on :8080.
//
//   node e2e/regression/v1_wave2_fixes.mjs
import { boot, nav, setTheme, mainText, errsOf } from "./_harness.mjs";

const results = [];
const check = (name, ok, extra = "") => { results.push({ name, ok }); console.log(`  ${ok ? "✓" : "✗"} ${name}${extra ? " — " + extra : ""}`); };

const clickSettingsTab = async (page, name) => {
  await nav(page, "/settings", 1200);
  await page.evaluate((n) => {
    const strip = document.querySelector(".settings-page .set-tab-strip");
    const t = [...(strip ? strip.querySelectorAll("button") : [])].find((b) => b.textContent.trim() === n);
    if (t) t.click();
  }, name);
  await page.waitForTimeout(800);
};

const { browser, context, page, errors } = await boot();
try {
  // --- Group J: custom-page list rows carry a date sub-line ---
  await nav(page, "/p/side-hustle", 2400);
  const sh = (await mainText(page)).toLowerCase();
  check("custom list rows show a date sub-line (not five bare dupes)",
    /\b(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d{1,2},\s+20\d\d/.test(sh));

  // --- Group J: a pure-expense "costs" flow chart plots positive magnitudes ---
  await nav(page, "/p/priya-business", 2400);
  const negInCosts = await page.evaluate(() => {
    // The Shop-costs chart axis labels should not carry minus signs now.
    const tiles = [...document.querySelectorAll("*")].filter((e) => e.textContent && e.textContent.trim() === "Shop costs (live, 12 months)");
    if (!tiles.length) return "no-tile";
    // Walk up to the tile card, then scan its SVG/axis text for a leading minus.
    let card = tiles[0];
    for (let i = 0; i < 6 && card && !card.querySelector("svg"); i++) card = card.parentElement;
    if (!card) return "no-card";
    const txt = card.innerText || "";
    return /[-−]\s?\$/.test(txt) ? "HAS_NEG" : "positive";
  });
  check("costs chart plots positive magnitudes (no negative $ axis)", negInCosts === "positive", String(negInCosts));

  // --- Group I: backend off by default, live-action buttons hidden ---
  await clickSettingsTab(page, "Cloud");
  const cloud = await page.evaluate(() => {
    const txt = (document.querySelector("#main") || document.body).innerText.toLowerCase();
    const sw = document.querySelector("[role=switch]");
    return {
      off: txt.includes("backend off") || txt.includes("fully local"),
      checked: sw ? sw.getAttribute("aria-checked") : "none",
      test: txt.includes("test connection"),
      sync: txt.includes("sync now"),
      upload: txt.includes("upload key"),
    };
  });
  check("Cloud backend defaults OFF", cloud.off && cloud.checked === "false");
  check("Cloud Test/Sync/Upload hidden while off", !cloud.test && !cloud.sync && !cloud.upload);

  // --- Group I: single-language display picker is hidden ---
  await clickSettingsTab(page, "Advanced");
  const adv = await page.evaluate(() => {
    const sel = document.querySelector(".settings-page select[title='Display language'], .settings-page select[aria-label='Display language']");
    const txt = (document.querySelector("#main") || document.body).innerText;
    return { noSelect: !sel, hint: txt.includes("only language installed") };
  });
  check("one-option language picker hidden, hint shown", adv.noSelect && adv.hint);

  // --- Light-theme sweep across the custom pages (parity) ---
  await setTheme(page, "light");
  await nav(page, "/p/side-hustle", 1800);
  await nav(page, "/p/priya-business", 1800);
  check("light-theme custom pages render without error", errsOf(errors) === "none", errsOf(errors));

  const failed = results.filter((r) => !r.ok);
  console.log(failed.length ? `\nFAIL: v1_wave2_fixes — ${failed.length} check(s) failed` : "\nPASS: v1_wave2_fixes — all second-wave fixes hold");
  if (failed.length) process.exitCode = 1;
} finally {
  await context.close();
  await browser.close();
}
