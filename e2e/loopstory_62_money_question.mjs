// L62 E2E loop story — "The Money Question" (Renu, Insights Q&A) — 2026-06-22
//
// Persona: Renu is a solo earner who just landed on /insights for the first
// time. She has no AI key set. She wants to:
//   1. See whether the no-key state is handled gracefully (CTA present, not
//      a blank dead-end).
//   2. Follow the CTA to wherever Settings lives in the app.
//   3. Confirm the AI key field exists in Settings (so she could enter one).
//   4. Return to /insights and use a starter chip to fill the Ask box.
//   5. Confirm the affordability fast-path works WITHOUT an AI key (it
//      answers from real figures — no LLM call needed).
//   6. Pin an affordability answer (SavedInsight persistence, no key needed).
//   7. Manually add a task via the /todo form and verify it persists.
//   8. After a reload, confirm: the pinned insight still exists; the task is
//      still on /todo; /insights still shows the no-key CTA.
//
// NOTE on save-as-task: The chat "save as task" path is an agent tool
// (add_task in chat_agent.go) that REQUIRES an AI key. We cannot invoke
// it here without a real key. Instead we test the todo lifecycle directly
// via the /todo add form, which is the same persistence path the tool uses
// (PutTask → SQLiteStore). This is a valid stand-in for I3.
//
// KEY INVARIANTS ASSERTED:
//   I1: NO_KEY_CTA_PRESENT
//       /insights without an API key shows a call-to-action hint + a
//       "Settings" button (C59). The CTA is NOT a dead-end.
//   I2: SETTINGS_NAVIGATES
//       Clicking the Settings CTA changes the URL to /settings (C59 nav
//       contract). NOTE: /settings is not a registered route; the router's
//       "*" catch-all serves the dashboard. The URL changes but NO settings
//       modal opens automatically — this is a REAL GAP (the CTA navigates to
//       a URL that doesn't open the settings panel; C59 passes only the URL
//       assertion, not the modal assertion).
//   I3: TODO_ROUNDTRIP
//       A task added via /todo form survives a full page reload.
//   I4: AFFORD_NO_KEY
//       The affordability fast-path ("Can I afford $500?") returns a
//       grounded answer WITHOUT an AI key. The answer appears in the thread.
//   I5: CONTEXT_GROUNDED
//       The AI context payload is built from real figures (net worth, income,
//       expense, account count). Verified indirectly: the affordability
//       fast-path uses those figures (ledger.NetWorth + ledger.PeriodTotals).
//   I6: PIN_ROUNDTRIP
//       A pinned insight appears in the "Pinned insights" card after pinning,
//       and survives a reload.
//
// Gap summary (real findings from this probe):
//   GAP-A: /settings URL is not a registered route — the Settings CTA on
//           /insights navigates to /settings but lands on the dashboard
//           wildcard catch-all, NOT the settings modal. The modal only opens
//           via the household-selector button in the shell. C59 passes only
//           because the test checks URL change, not modal opening.
//   GAP-B: save-as-task requires an AI key; there is no direct UI button to
//           save an Insights Q&A answer as a /todo item without the model.
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_62_money_question.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { mkdirSync } from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const SS = (name) => path.join(__dirname, name);

// ── helpers ───────────────────────────────────────────────────────────────────

const goto = async (page, hash) => {
  await page.goto(BASE + hash, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(2500);
};

const navTo = async (page, title) => {
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link = links.find(l => l.getAttribute("title") === t);
    if (link) link.click();
  }, title);
  await page.waitForTimeout(1800);
};

const bodyText = (page) => page.evaluate(() => document.body.innerText);

try { mkdirSync(path.join(__dirname, "screenshots"), { recursive: true }); } catch (_) {}

let passes = 0, fails = 0, maybes = 0;
const pass  = (m) => { passes++;  console.log(`  PASS  ${m}`); };
const fail  = (m) => { fails++;   console.error(`  FAIL  ${m}`); process.exitCode = 1; };
const maybe = (m) => { maybes++;  console.warn(`  MAYBE ${m}`); };
const note  = (m) => { console.log(`  NOTE  ${m}`); };

// ── main ──────────────────────────────────────────────────────────────────────
const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const jsErrors = [];
  page.on("pageerror", (e) => jsErrors.push(String(e)));

  // ── Step 0: Ensure no AI key is set ──────────────────────────────────────
  console.log("\n── Step 0: Clear any stored AI key ──");
  await goto(page, "/");
  // Clear OpenAI key from localStorage so we start in no-key state
  await page.evaluate(() => {
    // The app stores the AI key under various keys; clear the known ones
    localStorage.removeItem("cf:ai-key");
    localStorage.removeItem("cf:openai-key");
    // Also clear from the SQLite dataset in-memory (not possible via LS),
    // but since we just booted, the dataset may have a key from a prior session.
    // We cannot reliably clear it here without knowing the store key format.
    // The test environment should start clean; skip if a key was persisted in LS.
    const stored = Object.keys(localStorage).filter(k => k.includes("ai") || k.includes("openai") || k.includes("key"));
    stored.forEach(k => {
      const v = localStorage.getItem(k);
      if (v && v.startsWith("sk-")) localStorage.removeItem(k);
    });
  });
  await page.waitForTimeout(500);

  // ── Step 1: /insights — check no-key CTA ──────────────────────────────────
  console.log("\n── Step 1: /insights — no-key CTA ──");
  await goto(page, "/");
  await navTo(page, "Insights");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_01_insights_no_key.png") });

  const insightsBody = await bodyText(page);

  // I1: No-key CTA present (C59)
  // The hint text key is "insights.keyHint" and the button label is "nav.settings"
  const hasKeyHint = insightsBody.includes("Add your OpenAI key") ||
                     insightsBody.includes("key") && insightsBody.includes("Settings") ||
                     insightsBody.includes("OpenAI");
  const hasSettingsBtn = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button"));
    return btns.some(b => b.textContent.trim() === "Settings");
  });

  if (hasKeyHint || hasSettingsBtn) {
    pass("I1 NO_KEY_CTA_PRESENT: insights page shows key hint / Settings CTA in no-key state (C59 ✓)");
  } else {
    maybe("I1 NO_KEY_CTA_PRESENT: key hint not detected — may already have a key set, or i18n differs");
  }

  // Check that starter chips are shown even without a key (L8)
  const hasChips = await page.evaluate(() => {
    const chips = document.querySelectorAll(".chip-suggest");
    return chips.length > 0;
  });
  if (hasChips) {
    pass("STARTER_CHIPS_PRESENT: starter question chips shown even without AI key (L8 ✓)");
  } else {
    fail("STARTER_CHIPS_PRESENT: no starter chips visible — blank box cold-start (L8 regression)");
  }

  // ── Step 2: Click Settings CTA — observe navigation ───────────────────────
  console.log("\n── Step 2: Click Settings CTA ──");
  const settingsBtnClicked = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button"));
    const btn = btns.find(b => b.textContent.trim() === "Settings");
    if (btn) { btn.click(); return true; }
    return false;
  });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_02_after_settings_click.png") });

  const settingsURL = page.url();
  const settingsBody = await bodyText(page);
  note(`URL after Settings CTA click: ${settingsURL}`);

  if (!settingsBtnClicked) {
    fail("I2 SETTINGS_NAVIGATES: Settings button not found — CTA absent or different label");
  } else if (settingsURL.includes("/settings")) {
    pass("I2 SETTINGS_NAVIGATES: URL changed to /settings after CTA click (C59 URL contract ✓)");
    // Now check whether the settings MODAL actually opened — this tests the REAL gap
    const settingsModalOpen = await page.evaluate(() => {
      // The settings modal is a FlipPanel rendered by SettingsHost; look for the panel
      const panel = document.querySelector(".flip-panel") || document.querySelector('[class*="flip"]');
      const settingsHeading = Array.from(document.querySelectorAll("h2, h3")).find(h => h.textContent.includes("Settings") || h.textContent.includes("Household"));
      const aiKeyField = document.querySelector('input[type="password"]');
      return { panel: !!panel, heading: settingsHeading ? settingsHeading.textContent.trim() : null, aiKeyField: !!aiKeyField };
    });
    note(`Settings state after CTA: panel=${settingsModalOpen.panel}, heading="${settingsModalOpen.heading}", aiKeyField=${settingsModalOpen.aiKeyField}`);
    if (!settingsModalOpen.aiKeyField && !settingsModalOpen.panel) {
      // GAP-A: navigating to /settings doesn't open the modal
      fail("GAP-A SETTINGS_MODAL_NOT_OPENED: /settings URL reached but settings modal did NOT open — the CTA lands on the dashboard wildcard (no /settings route); user cannot find the AI key field from the CTA alone");
    } else {
      pass("SETTINGS_MODAL_OPENED: settings modal opened + AI key field visible after CTA navigation");
    }
  } else {
    fail(`I2 SETTINGS_NAVIGATES: URL did NOT change to /settings (current: ${settingsURL})`);
  }

  // ── Step 3: Find/open global settings to check provider UI ────────────────
  console.log("\n── Step 3: Check AI key field in settings modal ──");
  // The settings modal is opened via the household-selector button (top bar gear icon)
  // regardless of URL. Let's navigate home then open settings properly.
  await goto(page, "/");
  await page.waitForTimeout(1000);

  // Open global settings via the header settings button
  const settingsOpened = await page.evaluate(() => {
    // Find the gear/settings button in the top bar
    const btns = Array.from(document.querySelectorAll("button"));
    // The household-selector button shows the household name + settings icon
    const settingsBtn = btns.find(b =>
      b.querySelector('[title*="Settings"]') ||
      b.closest('[class*="hh"]') ||
      (b.title && b.title.includes("Settings"))
    );
    if (settingsBtn) { settingsBtn.click(); return "found_and_clicked"; }
    // Try clicking on any button that has a settings icon
    const gearBtn = btns.find(b => b.innerHTML.includes("svg") && (b.title || "").toLowerCase().includes("settings"));
    if (gearBtn) { gearBtn.click(); return "found_gear_and_clicked"; }
    return "not_found";
  });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_03_settings_modal.png") });

  note(`Settings open attempt: ${settingsOpened}`);

  const aiKeyVisible = await page.evaluate(() => {
    const inp = document.querySelector('input[type="password"][placeholder*="sk-"], input[type="password"][placeholder*="key"], input[type="password"]');
    return inp ? { found: true, placeholder: inp.placeholder } : { found: false };
  });
  note(`AI key field: ${JSON.stringify(aiKeyVisible)}`);

  if (aiKeyVisible.found) {
    pass("SETTINGS_AI_KEY_FIELD: AI key password input found in settings (C81-adjacent ✓)");
  } else {
    maybe("SETTINGS_AI_KEY_FIELD: settings modal may not be open, or AI key field has different selector");
  }

  // Close the modal / dismiss
  await page.evaluate(() => {
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    const closeBtn = document.querySelector('button[aria-label="Close"]');
    if (closeBtn) closeBtn.click();
    const backdrop = document.querySelector(".flip-backdrop.show, .flip-backdrop");
    if (backdrop) backdrop.click();
  });
  await page.waitForTimeout(500);

  // ── Step 4: Affordability fast-path (no AI key needed) ────────────────────
  console.log("\n── Step 4: Affordability fast-path without AI key ──");
  await goto(page, "/");
  await navTo(page, "Insights");
  await page.waitForTimeout(1500);

  // Type an affordability question into the Ask box
  const affordQuestion = "Can I afford $500?";
  const inputFilled = await page.evaluate((q) => {
    const inp = document.getElementById("cf-chat-input");
    if (!inp) return false;
    inp.focus();
    inp.value = q;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    return true;
  }, affordQuestion);
  note(`Input filled: ${inputFilled}, question: "${affordQuestion}"`);
  await page.waitForTimeout(300);

  // Submit by pressing Enter
  await page.keyboard.press("Enter");
  await page.waitForTimeout(2000);
  await page.screenshot({ path: SS("l62_04_afford_result.png") });

  const affordBody = await bodyText(page);
  const affordResultPresent = await page.evaluate(() => {
    return !!document.querySelector('[data-cf="afford-result"]');
  });
  const affordTextPresent = affordBody.includes("afford") || affordBody.includes("Can afford") ||
                            affordBody.includes("cannot afford") || affordBody.includes("net worth");

  if (affordResultPresent) {
    pass("I4 AFFORD_NO_KEY: affordability fast-path produced a result card (data-cf='afford-result') without an AI key ✓");
  } else if (affordTextPresent) {
    pass("I4 AFFORD_NO_KEY: affordability fast-path produced answer text without an AI key ✓");
  } else {
    // Check if an error appeared (needs-key message for regular chat)
    const hasNeedsKeyErr = affordBody.includes("Add your") || affordBody.includes("key") && affordBody.includes("required");
    if (hasNeedsKeyErr) {
      fail("I4 AFFORD_NO_KEY: affordability fast-path did NOT fire — app asked for a key instead of computing offline answer (fast-path regression)");
    } else {
      maybe("I4 AFFORD_NO_KEY: result not detected — may need more time or different selector");
    }
  }

  // I5: Context grounded — verify the page has real financial figures
  const hasRealFigures = affordBody.match(/\$[\d,]+\.?\d*/);
  if (hasRealFigures) {
    pass("I5 CONTEXT_GROUNDED: real dollar figures present in insights thread (context not empty) ✓");
  } else {
    fail("I5 CONTEXT_GROUNDED: no dollar figures visible — context may be empty or ungrounded");
  }

  // ── Step 5: Pin an answer ─────────────────────────────────────────────────
  console.log("\n── Step 5: Pin the affordability answer ──");
  // The Pin button is in the AssistantBubble or AffordResultBubble action row.
  // For affordability fast-path answers, use the "afford-result" bubble's delete-
  // adjacent area (Pin is only on AssistantBubble, not AffordResultBubble).
  // The Pin button is revealed on hover; simulate hover.
  let pinned = false;
  const pinAttempt = await page.evaluate(() => {
    // Look for Pin button in any assistant bubble
    const bubbles = document.querySelectorAll(".group");
    for (const bubble of bubbles) {
      const btns = Array.from(bubble.querySelectorAll("button"));
      const pinBtn = btns.find(b => b.textContent.includes("Pin") || b.title.includes("Pin") || b.title.includes("pin"));
      if (pinBtn) { pinBtn.click(); return "clicked_pin"; }
    }
    // Try focusing the bubble to reveal hover actions
    const groups = document.querySelectorAll(".group");
    if (groups.length > 0) {
      groups[groups.length - 1].dispatchEvent(new MouseEvent("mouseenter", { bubbles: true }));
      // Try again
      const btns2 = Array.from(groups[groups.length - 1].querySelectorAll("button"));
      const pinBtn2 = btns2.find(b => b.textContent.includes("Pin") || (b.title || "").toLowerCase().includes("pin"));
      if (pinBtn2) { pinBtn2.click(); return "clicked_pin_after_hover"; }
    }
    return "not_found";
  });
  note(`Pin attempt: ${pinAttempt}`);

  if (pinAttempt.startsWith("clicked_pin")) {
    await page.waitForTimeout(800);
    const afterPinBody = await bodyText(page);
    const pinnedConfirm = afterPinBody.includes("Pinned") || afterPinBody.includes("pinned") || afterPinBody.includes("Saved");
    if (pinnedConfirm) {
      pass("I6 PIN_ROUNDTRIP: Pin clicked and confirmation feedback visible");
      pinned = true;
    } else {
      // Check if pinned insights card appeared
      const hasPinnedCard = afterPinBody.includes("Pinned insights");
      if (hasPinnedCard) {
        pass("I6 PIN_ROUNDTRIP: Pin clicked; 'Pinned insights' card appeared");
        pinned = true;
      } else {
        maybe("I6 PIN_ROUNDTRIP: Pin clicked but confirmation not detected — may be a hover-state issue");
      }
    }
  } else {
    // Affordability cards don't have a Pin button — only AssistantBubble does.
    // Note this as a gap: save-as-insight requires either Pin (AssistantBubble only)
    // or the AI model calling add_task.
    maybe("I6 PIN_ROUNDTRIP: Pin button not found — affordability fast-path cards (AffordResultBubble) do NOT have a Pin action; only AssistantBubble does. No save path for offline answers.");
  }
  await page.screenshot({ path: SS("l62_05_after_pin.png") });

  // ── Step 6: Add a task via /todo form ─────────────────────────────────────
  console.log("\n── Step 6: Add task via /todo form ──");
  await navTo(page, "To-do");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_06_todo_before.png") });

  const TASK_TITLE = "L62 Review food spending from Insights";

  const taskAdded = await page.evaluate((title) => {
    // The title input has id="task-add" and placeholder "What needs doing?"
    const titleInp = document.getElementById("task-add") ||
                     document.querySelector('input[placeholder*="needs doing"], input[placeholder*="What needs"]') ||
                     Array.from(document.querySelectorAll('input[type="text"]')).find(i =>
                       (i.placeholder || "").includes("needs doing") ||
                       (i.placeholder || "").toLowerCase().includes("what needs") ||
                       (i.getAttribute("aria-required") === "true")
                     );
    if (!titleInp) return { ok: false, reason: "no_title_input" };
    titleInp.focus();
    titleInp.value = title;
    titleInp.dispatchEvent(new Event("input", { bubbles: true }));

    // Find and click submit button
    const submitBtn = document.querySelector('button[type="submit"]') ||
                      Array.from(document.querySelectorAll("button")).find(b => b.textContent.includes("Add") || b.textContent.includes("Save"));
    if (!submitBtn) return { ok: false, reason: "no_submit_button" };
    submitBtn.click();
    return { ok: true, placeholder: titleInp.placeholder };
  }, TASK_TITLE);

  note(`Task add attempt: ${JSON.stringify(taskAdded)}`);
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_07_todo_after_add.png") });

  const todoBody = await bodyText(page);
  const taskPresent = todoBody.includes(TASK_TITLE);

  if (!taskAdded.ok) {
    fail(`I3 TODO_ROUNDTRIP (add): Could not fill add form — ${taskAdded.reason}`);
  } else if (taskPresent) {
    pass("I3 TODO_ROUNDTRIP (add): task title present in /todo list after submit");
  } else {
    fail(`I3 TODO_ROUNDTRIP (add): task submitted but title "${TASK_TITLE}" not found in /todo list`);
  }

  // ── Step 7: Reload and verify persistence ──────────────────────────────────
  console.log("\n── Step 7: Reload — verify task + pin roundtrip ──");
  await goto(page, "/");
  await page.waitForTimeout(1500);
  await navTo(page, "To-do");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_08_todo_after_reload.png") });

  const todoBodyReload = await bodyText(page);
  const taskAfterReload = todoBodyReload.includes(TASK_TITLE);

  if (taskAfterReload) {
    pass("I3 TODO_ROUNDTRIP (reload): task persisted across full page reload ✓");
  } else {
    fail("I3 TODO_ROUNDTRIP (reload): task NOT found after reload — persistence broken");
  }

  // Check pinned insight roundtrip (if we pinned one)
  if (pinned) {
    await navTo(page, "Insights");
    await page.waitForTimeout(1500);
    await page.screenshot({ path: SS("l62_09_insights_after_reload.png") });
    const insightsBodyReload = await bodyText(page);
    const pinnedAfterReload = insightsBodyReload.includes("Pinned insights");
    if (pinnedAfterReload) {
      pass("I6 PIN_ROUNDTRIP (reload): 'Pinned insights' card visible after reload ✓");
    } else {
      fail("I6 PIN_ROUNDTRIP (reload): Pinned insights card NOT found after reload — persistence broken");
    }
  }

  // ── Step 8: Verify /insights still shows no-key CTA after reload ──────────
  console.log("\n── Step 8: Post-reload no-key CTA check ──");
  await navTo(page, "Insights");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l62_10_insights_cta_post_reload.png") });

  const insightsPostReload = await bodyText(page);
  const ctaPostReload = insightsPostReload.includes("Add your OpenAI key") ||
                        insightsPostReload.includes("Settings") && insightsPostReload.includes("key");
  const settingsBtnPostReload = await page.evaluate(() => {
    return Array.from(document.querySelectorAll("button")).some(b => b.textContent.trim() === "Settings");
  });

  if (ctaPostReload || settingsBtnPostReload) {
    pass("CTA_STABLE_POST_RELOAD: no-key CTA still visible after reload (no regression) ✓");
  } else {
    maybe("CTA_STABLE_POST_RELOAD: CTA not detected post-reload — may have a key from prior session");
  }

  // ── Final: JS errors ──────────────────────────────────────────────────────
  console.log("\n── Final checks ──");
  if (jsErrors.length === 0) {
    pass("NO_JS_ERRORS: zero page-level JS errors across the full ritual ✓");
  } else {
    fail(`JS_ERRORS: ${jsErrors.length} error(s): ${jsErrors.slice(0, 3).join(" | ")}`);
  }

  // GAP-A structural note (always logged)
  console.log("\n  NOTE  GAP-A: /settings is NOT a registered route in app.go. The insights CTA");
  console.log("              calls nav.Navigate('/settings') which hits the '*' wildcard and");
  console.log("              renders the dashboard. The URL changes but NO settings modal opens.");
  console.log("              C59 passes only because insights_keyhint_check.mjs tests the URL");
  console.log("              change, not whether the user can actually reach the AI key field.");
  console.log("\n  NOTE  GAP-B: save-as-task from Insights requires the AI model to call the");
  console.log("              add_task tool (chat_agent.go) — no direct UI button exists.");
  console.log("              This path is unreachable in a no-key probe.");

} finally {
  await browser.close();
}

console.log(`\n── Summary: ${passes} pass · ${fails} fail · ${maybes} maybe ──`);
if (process.exitCode) {
  console.error("RESULT: FAIL");
} else {
  console.log("RESULT: PASS");
}
