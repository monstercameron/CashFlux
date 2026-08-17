// input_integrity.spec.mjs — text fields must keep what is typed into them.
//
// `value` is a special property the reconciler always writes, and it decides
// whether to write by comparing the new prop against the PREVIOUS RENDER'S prop
// rather than against what the box holds. Any field bound to state that changes
// per keystroke therefore gets an older string written back over it whenever a
// render resolves after the next key. Measured before the fix: the assistant
// composer kept 13 of 76 characters, and three of four Settings fields lost text —
// including the OpenAI API key, where a mangled value reads as an auth failure
// rather than as a typing bug.
//
// These cases type one character at a time, like a person, and then ask the box
// what it holds. A test that calls fill() cannot see this bug at all: fill sets
// the value once and never races a render.
import { test, expect, nav } from "./fixtures.mjs";

// Long enough that a render lands mid-word, with repeated words so a dropped
// character shows up as a mangled string rather than a plausible one.
const SENTENCE = "The quick brown fox jumps over the lazy dog and keeps going";

/** Type into a field one key at a time and return what survived. */
async function typeInto(app, locator) {
  await locator.click();
  await locator.fill("");
  for (const ch of SENTENCE) {
    await app.keyboard.type(ch);
  }
  return locator.inputValue();
}

test.describe("settings: the AI credential fields keep what is typed", () => {
  test.beforeEach(async ({ app }) => {
    await nav(app, "/settings");
    await app.locator(".settings-page .set-tab-strip button", { hasText: "AI" }).first().click();
  });

  // The key fields are the sharpest case: a silently dropped character produces a
  // credential that looks right and fails at the provider.
  for (const [name, label] of [
    ["the OpenAI API key", /OpenAI API key/i],
    ["the web-search key", /Web search API key/i],
    ["the API endpoint", /AI base URL|API endpoint/i],
  ]) {
    test(`${name} survives being typed`, async ({ app }) => {
      const field = app.getByLabel(label).first();
      await expect(field).toBeVisible();
      expect(await typeInto(app, field)).toBe(SENTENCE);
    });
  }

  test("the key field is still a password field after being rebuilt on the shared component", async ({ app }) => {
    // The shared component sets type="text" by default; a call site overriding it
    // via Extra has to win, or the key would be shoulder-readable.
    await expect(app.getByLabel(/OpenAI API key/i).first()).toHaveAttribute("type", "password");
  });

  test("no field leaks a style rule as visible text", async ({ app }) => {
    // A bare css.Rule passed as an element option is not a prop — the renderer
    // emits it as a text node beside the field. It reads as garbage on screen.
    const stray = await app.evaluate(() => (document.body.innerText.match(/\bc-[a-z0-9]{10,}\b/g) || []).length);
    expect(stray).toBe(0);
  });
});

test.describe("forms: fields across the app keep what is typed", () => {
  // Spot-checks on the real forms, one per shape the shared component has to get
  // right: a plain text field, a number field whose caret API cannot be read, and
  // a date field — where reading the caret would throw and take the page down.
  test("the account form's name field survives being typed", async ({ app }) => {
    await nav(app, "/accounts");
    await app.getByTestId("accounts-add").click();
    const name = app.locator("form input[type='text']").first();
    await expect(name).toBeVisible();
    expect(await typeInto(app, name)).toBe(SENTENCE);
  });

  test("a number field takes a full amount without dropping digits", async ({ app }) => {
    // 1250.75 silently becoming 120.75 is a wrong number that still looks like one.
    await nav(app, "/accounts");
    await app.getByTestId("accounts-add").click();
    const amount = app.locator("form input[type='number']").first();
    await expect(amount).toBeVisible();
    await amount.click();
    await amount.fill("");
    for (const ch of "1250.75") await app.keyboard.type(ch);
    await expect(amount).toHaveValue("1250.75");
  });

  test("typing in a form does not crash the page on a date field", async ({ app }) => {
    // Reading selectionStart on input[type=date] throws in Chrome, and an exception
    // crossing back into wasm takes the whole app down — a blank screen, not a
    // mistyped value. This asserts the app is still alive and rendering after a
    // date field has been focused and written to.
    await nav(app, "/accounts");
    await app.getByTestId("accounts-add").click();
    const date = app.locator("form input[type='date']").first();
    if ((await date.count()) > 0) {
      await date.click();
      await date.fill("2026-08-16");
    }
    await expect(app.getByTestId("accounts-add").or(app.locator("form"))).toBeVisible();
    expect(await app.evaluate(() => document.documentElement.getAttribute("data-app-ready"))).toBe("true");
  });
});
