// gapfeatures.spec.mjs — e2e regressions for the "competitive gap" feature wave:
// local-first additions (no external paid service) that close gaps vs comps —
// unusual-charge alerts, goal growth projection, cancel/negotiate helper, cash-flow
// forecast + safe-to-spend, tax/investment reports, and assistant voice input.
// Each test drives the seeded dataset (clock pinned to FIXED_NOW by boot) and
// asserts the real result.
import { test, expect, nav, mainText } from "./fixtures.mjs";

test.describe("gap features", () => {
  test("unusual-charge alert: a merchant billing far above its own normal surfaces", async ({ app }) => {
    // The sample seeds a $68 Blue Bottle Coffee charge against a ~$7 baseline; the
    // on-device unusual-charge detector should flag it in the Notification Center.
    await nav(app, "/notifications");
    const text = await mainText(app);
    expect(text).toMatch(/unusual charge at blue bottle coffee/i);
    // The body states the charge vs the payee's typical amount.
    expect(text).toMatch(/\$68\.00/);
    expect(text).toMatch(/usual \$7\.35/i);
  });
});
