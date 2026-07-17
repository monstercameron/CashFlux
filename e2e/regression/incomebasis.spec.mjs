// incomebasis.spec.mjs — regressions for the "Income to budget with" basis: the
// staged modal must persist its selection through save/reopen/reload, drive the
// page income immediately, and survive a passcode-locked (encrypted-at-rest)
// reboot — the boot path where the prefs atom used to seed from an empty store.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("budgets: income-basis modal persists selections", () => {
  // Cam report 2026-07-17: "Income to budget with doesn't persist when making a
  // selection". Locks the whole loop: pick a basis in the modal, Save, and the
  // choice must (a) drive the page's income figure immediately and (b) survive
  // reopening the modal and a full reload.
  test("fixed-income basis saves, updates the page, and survives reopen + reload", async ({ app }) => {
    await nav(app, "/budgets");
    const openBtn = app.locator('[data-testid="budgets-basis-open"]').first();
    await openBtn.scrollIntoViewIfNeeded();
    await openBtn.click();
    const mode = app.locator('[data-testid="budgets-zbb-income-mode"]');
    await expect(mode).toBeVisible();
    await mode.selectOption("fixed");
    const fixed = app.locator('[data-testid="budgets-zbb-fixed-amount"]');
    await expect(fixed).toBeVisible(); // the mode selection itself must stick
    await fixed.fill("6000");
    await app.getByRole("button", { name: /^save$/i }).last().click();
    await app.waitForTimeout(600);
    // The saved basis drives the page immediately — no reload needed.
    await expect(app.locator("#main")).toContainText("$6,000.00");
    // Reopen: the staged draft reseeds from the SAVED prefs.
    await openBtn.click();
    await expect(mode).toHaveValue("fixed");
    await expect(app.locator('[data-testid="budgets-zbb-fixed-amount"]')).toHaveValue("6000");
    await app.keyboard.press("Escape");
    // Reload FROM THE ROOT: the choice persisted to the dataset (RequestPersist),
    // not just memory. (A sub-route reload depends on the server's history
    // fallback / SW shell path — not what this test is about.)
    await app.evaluate(() => history.pushState({}, "", "/"));
    await app.reload();
    await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45000 });
    await nav(app, "/budgets");
    await expect(app.locator("#main")).toContainText("$6,000.00");
    await app.locator('[data-testid="budgets-basis-open"]').first().click();
    await expect(mode).toHaveValue("fixed");
  });
});

test.describe("budgets: income basis survives a passcode-locked reboot", () => {
  // Cam 2026-07-17 root cause: with an app-lock passcode, the dataset is an
  // encrypted envelope at boot, the lock screen renders first, and the prefs
  // atom seeded from an EMPTY store (defaults). hydrateFromPasscode imported
  // the dataset but never re-seeded prefs — so every locked boot showed the
  // default income basis ("all income") even though the saved value sat in
  // the store, and the next prefs write persisted those defaults over it.
  test("set passcode → save basis → reload → unlock → basis intact", async ({ app }) => {
    const PIN = "cashflux77"; // 8+ chars, two classes (digits-only 6-codes grade Weak and are rejected)
    await nav(app, "/settings");
    await app.locator(".set-tab-strip").getByText("Advanced", { exact: true }).click();
    await app.waitForTimeout(500);
    await app.getByText(/Set passcode lock/i).first().click();
    await app.locator("#cf-al-pass").fill(PIN);
    await app.locator("#cf-al-confirm").fill(PIN);
    await app.locator("#cf-al-ok").click();
    // The setup dialog hides (display:none) on success — a lingering VISIBLE
    // dialog means enabling failed; surface its own error text instead of a
    // mute pointer-interception timeout downstream.
    await app.locator("#cf-applock-setup").waitFor({ state: "hidden", timeout: 15000 }).catch(async () => {
      const msg = await app.locator("#cf-al-err").innerText().catch(() => "");
      throw new Error("passcode setup did not close: " + (msg || "no error text shown"));
    });
    await app.waitForTimeout(500);
    // Enabling may lock immediately — unlock via the GATE's input specifically
    // (the setup dialog's own passcode field shares the same aria-label).
    const gate0 = app.locator("#cf-applock-gate input").first();
    if (await gate0.count()) { await gate0.fill(PIN); await gate0.press("Enter"); await app.waitForTimeout(1500); }
    await nav(app, "/budgets");
    const openBtn = app.locator('[data-testid="budgets-basis-open"]').first();
    await openBtn.scrollIntoViewIfNeeded();
    await app.waitForTimeout(900); // let entrance animations settle (stable-element click)
    await openBtn.click();
    const mode = app.locator('[data-testid="budgets-zbb-income-mode"]');
    await mode.selectOption("fixed");
    await app.locator('[data-testid="budgets-zbb-fixed-amount"]').fill("6000");
    await app.getByRole("button", { name: /^save$/i }).last().click();
    await app.waitForTimeout(3000); // encrypted write is async — let it commit
    // Reload from the root (a sub-route reload depends on the server's history
    // fallback / the SW shell path — not what this test is about), and wait on
    // the GATE itself: a locked boot's readiness signal is the gate appearing.
    await app.evaluate(() => history.pushState({}, "", "/"));
    await app.reload();
    const gi = app.locator("#cf-applock-gate input").first();
    await gi.waitFor({ state: "visible", timeout: 45000 });
    await gi.fill(PIN);
    await gi.press("Enter");
    await app.locator("#cf-applock-gate").waitFor({ state: "hidden", timeout: 10000 });
    await app.waitForTimeout(1500);
    await nav(app, "/budgets");
    const reopenBtn = app.locator('[data-testid="budgets-basis-open"]').first();
    await reopenBtn.scrollIntoViewIfNeeded();
    await app.waitForTimeout(900);
    await reopenBtn.click();
    await expect(mode).toHaveValue("fixed");
    await expect(app.locator('[data-testid="budgets-zbb-fixed-amount"]')).toHaveValue("6000");
  });
});
