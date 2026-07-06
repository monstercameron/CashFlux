// invariants.spec.mjs — cross-cutting guarantees that must hold on EVERY route,
// so one test kills a whole class of regressions rather than one bug:
//   1. The theme engine emits every core design token (non-empty) in BOTH themes.
//      A regression that stops emitting --text/--accent/etc. would silently fall
//      back to an undefined value (the "var(--fg) landmine" class of bug).
//   2. The page body never scrolls horizontally. Wide content (tables, charts)
//      must scroll inside its own overflow-x container; the document must not.
import { test, expect, ROUTES, nav, setTheme } from "./fixtures.mjs";

// Core tokens the runtime theme is contracted to define on documentElement.
const CORE_TOKENS = [
  "--text", "--text-dim", "--bg-base", "--bg-card", "--bg-elev",
  "--border", "--accent", "--up", "--down", "--warn", "--danger",
  "--font-ui", "--font-display",
];

test("core theme tokens are defined and non-empty in both themes", async ({ app }) => {
  for (const mode of ["dark", "light"]) {
    await setTheme(app, mode);
    const missing = await app.evaluate((tokens) => {
      const cs = getComputedStyle(document.documentElement);
      return tokens.filter((t) => !cs.getPropertyValue(t).trim());
    }, CORE_TOKENS);
    expect(missing, `${mode}: undefined/empty theme tokens`).toEqual([]);
  }
  await setTheme(app, "dark");
});

test("no route scrolls the page body horizontally", async ({ app }) => {
  test.setTimeout(300_000);
  const overflow = [];
  for (const [route] of ROUTES) {
    await nav(app, route);
    const bad = await app.evaluate(() => {
      const de = document.documentElement;
      // A few px tolerance for sub-pixel rounding / scrollbar gutters.
      return de.scrollWidth - de.clientWidth > 3 ? de.scrollWidth - de.clientWidth : 0;
    });
    if (bad) overflow.push(`${route}: body overflows by ${bad}px`);
  }
  expect(overflow, `horizontal overflow:\n${overflow.join("\n")}`).toEqual([]);
});
