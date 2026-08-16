// smart_plus_scan.spec.mjs — the SMART+ scan path, end to end, WITHOUT spending
// anything.
//
// The model call is intercepted at the network layer and answered with a canned
// completion, so this exercises the real code — request building, parsing,
// sign rejection, proposal application, "use these suggestions" — against a
// controlled reply. Everything except OpenAI itself is genuine.
//
// This exists because the scan shipped untested: the earlier specs only covered
// the strip's no-key state, which is exactly the half that could not break.
import { test, expect, nav } from "./fixtures.mjs";

const KEY_LABEL = /OpenAI API key/i;

/** Put a key in Settings so the strip offers a Scan button rather than "Connect a key". */
async function configureKey(app) {
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "AI" }).first().click();
  const field = app.getByLabel(KEY_LABEL).first();
  await expect(field).toBeVisible();
  await field.fill("sk-test-not-a-real-key");
  // The key is written through PutSettings on input; give the write a beat.
  await expect(field).toHaveValue("sk-test-not-a-real-key");
}

/**
 * Answer any chat-completion request with `content`, in OpenAI's response shape.
 * Returns a getter for how many times the model was called.
 */
async function mockModel(app, content) {
  const calls = { n: 0, bodies: [] };
  await app.route("**/chat/completions*", async (route) => {
    calls.n++;
    try {
      calls.bodies.push(JSON.parse(route.request().postData() || "{}"));
    } catch {
      calls.bodies.push(null);
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        id: "chatcmpl-test",
        object: "chat.completion",
        choices: [{ index: 0, message: { role: "assistant", content }, finish_reason: "stop" }],
        usage: { prompt_tokens: 10, completion_tokens: 10, total_tokens: 20 },
      }),
    });
  });
  // Saving a key also triggers unrelated background calls (the dashboard's
  // quote-of-the-day fires immediately), so callers must pick the
  // CATEGORIZATION request rather than assuming it is the first one.
  calls.categorize = () =>
    calls.bodies.find((b) => (b?.messages || []).some((m) => (m.content || "").includes("Categories:")));
  return calls;
}

async function openReview(app) {
  await nav(app, "/transactions");
  const btn = app.getByTestId("txn-review-btn");
  await expect(btn).toBeVisible();
  await btn.click();
  await expect(app.getByTestId("review-inbox")).toBeVisible();
}

/** The merchants the LOCAL sources could not place — the only ones a scan is for. */
async function gapMerchants(app) {
  const rows = app.locator('[data-tier="is-none"] .rvs-grp');
  const out = [];
  for (let i = 0; i < (await rows.count()); i++) {
    out.push((await rows.nth(i).locator(".rvs-grp-name strong").textContent()).trim());
  }
  return out;
}

test.describe("SMART+ scan", () => {
  test("offers a scan once a key is configured", async ({ app }) => {
    await configureKey(app);
    await openReview(app);

    const strip = app.getByTestId("review-scan-strip");
    await expect(strip).toBeVisible();
    // With a key present it must offer to SCAN, not to connect a key.
    await expect(app.getByTestId("review-scan-setup")).toHaveCount(0);
    await expect(app.getByTestId("review-scan")).toBeVisible();
    await expect(app.getByTestId("review-scan")).toContainText(/Scan \d+ charges?/);
    // And it states the cost before it can be clicked.
    await expect(strip).toContainText(/\$\d/);
  });

  test("a scan fills the merchants the local sources could not place", async ({ app }) => {
    await configureKey(app);
    await openReview(app);

    const gaps = await gapMerchants(app);
    test.skip(gaps.length === 0, "sample data left no unresolvable merchant to scan");

    // Answer with an assignment for ref 1 — whatever the first gap merchant is.
    const calls = await mockModel(app, "1 => Groceries | high\n");

    await app.getByTestId("review-scan").click();
    await expect(app.getByTestId("review-scan-title")).toContainText(/filled/i, { timeout: 20000 });

    expect(calls.n, "the scan must actually call the model").toBeGreaterThan(0);

    // The request carries the numbered merchants AND the category catalog with
    // full paths — without those the model cannot answer usefully.
    const sent = calls.categorize();
    expect(sent, "a categorization request must have been sent").toBeTruthy();
    const userMsg = (sent.messages || []).map((m) => m.content).join("\n");
    expect(userMsg).toContain("Categories:");
    expect(userMsg).toMatch(/1 \| /);

    // The proposal lands on the row as a real category.
    const filled = app.locator('[data-tier="is-none"] .rvs-cat').first();
    await expect.poll(async () => await filled.inputValue()).not.toBe("");

    // And accepting it arms the footer, leaving Confirm as the next click.
    await app.getByTestId("review-scan-use").click();
    await expect(app.getByTestId("review-selection")).toContainText(/\d+ merchants? · \d+ charges?/);
  });

  test("only the unresolved merchants are sent — a scan never re-derives a rule", async ({ app }) => {
    await configureKey(app);
    await openReview(app);
    const gaps = await gapMerchants(app);
    test.skip(gaps.length === 0, "no gap merchants in sample data");

    const calls = await mockModel(app, "1 => Groceries | high\n");
    await app.getByTestId("review-scan").click();
    await expect(app.getByTestId("review-scan-title")).toContainText(/filled/i, { timeout: 20000 });

    const sent = calls.categorize();
    expect(sent, "a categorization request must have been sent").toBeTruthy();
    const userMsg = (sent.messages || []).map((m) => m.content).join("\n");
    // Count the numbered transaction lines: one per GAP merchant, not one per
    // charge and not one per merchant in the queue.
    const numbered = (userMsg.match(/^\d+ \| /gm) || []).length;
    expect(numbered).toBe(Math.min(gaps.length, 40));
  });

  test("a refused or unparseable reply reports itself instead of failing silently", async ({ app }) => {
    await configureKey(app);
    await openReview(app);
    test.skip((await gapMerchants(app)).length === 0, "no gap merchants in sample data");

    await mockModel(app, "I'm sorry, I can't help with that.");
    await app.getByTestId("review-scan").click();

    // The strip must leave the scanning state and say what happened — a scan
    // that spends money and then shows nothing is the worst outcome.
    await expect(app.getByTestId("review-scan-strip")).toHaveAttribute("data-state", "done", {
      timeout: 20000,
    });
    await expect(app.getByTestId("review-scan-title")).toContainText(/filled 0|could not/i);
    // And it must still offer a retry: a dead end with nothing to click is
    // indistinguishable from the feature being broken.
    await expect(app.getByTestId("review-scan-again")).toBeVisible();
  });

  test("a transport error surfaces rather than hanging on 'Reading…'", async ({ app }) => {
    await configureKey(app);
    await openReview(app);
    test.skip((await gapMerchants(app)).length === 0, "no gap merchants in sample data");

    await app.route("**/chat/completions*", (route) => route.fulfill({ status: 500, body: "boom" }));
    await app.getByTestId("review-scan").click();

    await expect(app.getByTestId("review-scan-strip")).toHaveAttribute("data-state", "done", {
      timeout: 25000,
    });
  });
});
