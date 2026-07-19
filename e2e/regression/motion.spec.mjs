// motion.spec.mjs — v1.2.3 motion & interaction spec adherence ratchet.
//
// The July 19 motion spec fixes ONE duration scale (0/80/120/180/240/280/320/
// 450ms), THREE easing curves (standard 0.2,0,0,1 · enter 0.16,1,0.3,1 · exit
// 0.4,0,1,1), stagger caps (20ms/sibling, 100ms total), and hard behavioral
// rules (rows never translate or stagger on filtering, no overshoot/bounce, no
// confetti, a single sliding sidebar indicator). The suite runs with
// reducedMotion:reduce, so these tests assert the AUTHORED stylesheet rules
// (CSSOM — unaffected by the reduced-motion overrides) plus the DOM behaviors
// that survive reduced motion (indicator geometry, no-transform rows).
import { test, expect, nav } from "./fixtures.mjs";

// walkRules flattens every same-origin CSS rule (including those nested in
// @media / @supports) into [{selector, cssText, style}] descriptors evaluated
// in the page. Serialized back as plain data for assertions.
const COLLECT_RULES = `(() => {
  const out = [];
  const walk = (rules) => {
    for (const r of rules) {
      if (r.cssRules) { walk(r.cssRules); continue; }
      if (r.style) {
        out.push({
          selector: r.selectorText || (r.parentRule && r.parentRule.name) || "",
          keyframe: !!(r.parentRule && r.parentRule.type === CSSRule.KEYFRAMES_RULE),
          cssText: r.cssText,
          transitionDuration: r.style.transitionDuration || "",
          transitionDelay: r.style.transitionDelay || "",
          animationDuration: r.style.animationDuration || "",
          animationDelay: r.style.animationDelay || "",
          animationIterationCount: r.style.animationIterationCount || "",
        });
      }
    }
  };
  for (const sheet of document.styleSheets) {
    try { walk(sheet.cssRules); } catch (_) { /* cross-origin: none expected */ }
  }
  return out;
})()`;

// parseTimes turns a comma-separated CSS time list ("0.28s, 120ms") into
// milliseconds; entries that don't parse (e.g. unresolved var()) are skipped.
function parseTimes(list) {
  if (!list) return [];
  return list
    .split(",")
    .map((t) => t.trim())
    .map((t) => {
      const m = t.match(/^(-?\d*\.?\d+)(ms|s)$/);
      if (!m) return null;
      return m[2] === "s" ? parseFloat(m[1]) * 1000 : parseFloat(m[1]);
    })
    .filter((v) => v !== null);
}

// Deliberate attention cues, documented in the spec pass as non-routine motion:
// the two deep-link "you are here" flashes (0.9s one-shots).
const DURATION_WHITELIST = [".cf-jump-flash", ".deeplink-flash"];

test.describe("motion spec ratchet", () => {
  test("the v1.2.3 duration scale and easing curves are live on :root", async ({ app }) => {
    const tokens = await app.evaluate(() => {
      const cs = getComputedStyle(document.documentElement);
      const get = (n) => cs.getPropertyValue(n).trim().replace(/\s+/g, "");
      return {
        micro: get("--motion-micro"),
        fast: get("--motion-fast"),
        standard: get("--motion-standard"),
        layout: get("--motion-layout"),
        overlay: get("--motion-overlay"),
        data: get("--motion-data"),
        narrative: get("--motion-narrative"),
        easeStandard: get("--ease-standard"),
        easeEnter: get("--ease-enter"),
        easeExit: get("--ease-exit"),
      };
    });
    expect(tokens.micro).toBe("80ms");
    expect(tokens.fast).toBe("120ms");
    expect(tokens.standard).toBe("180ms");
    expect(tokens.layout).toBe("240ms");
    expect(tokens.overlay).toBe("280ms");
    expect(tokens.data).toBe("320ms");
    expect(tokens.narrative).toBe("450ms");
    expect(tokens.easeStandard).toBe("cubic-bezier(0.2,0,0,1)");
    expect(tokens.easeEnter).toBe("cubic-bezier(0.16,1,0.3,1)");
    expect(tokens.easeExit).toBe("cubic-bezier(0.4,0,1,1)");
  });

  test("no overshoot/elastic easing anywhere in the app stylesheet", async ({ app }) => {
    const rules = await app.evaluate(COLLECT_RULES);
    const offenders = [];
    for (const r of rules) {
      for (const m of r.cssText.matchAll(/cubic-bezier\(([^)]+)\)/g)) {
        const p = m[1].split(",").map((v) => parseFloat(v));
        if (p.length === 4 && (p[1] > 1.001 || p[3] > 1.001 || p[1] < -0.001 || p[3] < -0.001)) {
          offenders.push(`${r.selector}: cubic-bezier(${m[1]})`);
        }
      }
    }
    expect(offenders, "spec §2: a financial interface settles exactly once — no overshoot beziers").toEqual([]);
  });

  test("no routine transition or animation exceeds the 450ms ceiling", async ({ app }) => {
    const rules = await app.evaluate(COLLECT_RULES);
    const offenders = [];
    for (const r of rules) {
      if (DURATION_WHITELIST.some((w) => r.selector.includes(w))) continue;
      if (/infinite/.test(r.animationIterationCount)) continue; // loading loops (shimmer/pulse)
      for (const ms of [...parseTimes(r.transitionDuration), ...parseTimes(r.animationDuration)]) {
        if (ms > 451) offenders.push(`${r.selector}: ${ms}ms`);
      }
    }
    expect(offenders, "spec §2: nothing routine exceeds 450ms").toEqual([]);
  });

  test("stagger delays never exceed the 100ms total cap", async ({ app }) => {
    const rules = await app.evaluate(COLLECT_RULES);
    const offenders = [];
    for (const r of rules) {
      if (/infinite/.test(r.animationIterationCount)) continue;
      for (const ms of [...parseTimes(r.animationDelay), ...parseTimes(r.transitionDelay)]) {
        if (ms > 101) offenders.push(`${r.selector}: delay ${ms}ms`);
      }
    }
    expect(offenders, "spec §2: max total stagger is 100ms").toEqual([]);
  });

  test("no confetti rules or nodes exist", async ({ app }) => {
    const rules = await app.evaluate(COLLECT_RULES);
    expect(rules.filter((r) => /confetti/i.test(r.cssText)).map((r) => r.selector)).toEqual([]);
    await nav(app, "/notifications");
    expect(await app.locator("[class*='confetti']").count()).toBe(0);
  });
});

test.describe("interactive state behaviors", () => {
  test("ledger and list rows never stagger in or translate on hover", async ({ app }) => {
    const rules = await app.evaluate(COLLECT_RULES);
    // No entrance animation may target generic list rows (the old
    // wonder-row-enter cascade replayed on every filter re-render).
    const rowAnims = rules.filter(
      (r) => /\.rows \.row|\.list-rows \.row/.test(r.selector) && /animation/.test(r.cssText),
    );
    expect(rowAnims.map((r) => r.selector)).toEqual([]);
    // And a hovered row keeps transform:none (spec §3: rows do not translate).
    await nav(app, "/accounts");
    const row = app.locator("#main .row").first();
    if (await row.count()) {
      await row.hover();
      const transform = await row.evaluate((el) => getComputedStyle(el).transform);
      expect(transform, "rows must not translate on hover").toBe("none");
    }
  });

  test("the sidebar has ONE shared indicator that tracks the active item", async ({ app }) => {
    const ind = app.locator("#cf-rail-ind");
    await expect(ind).toBeAttached();
    // Navigate between two rail pages and assert the single bar re-points to the
    // active item's geometry each time (top/height within a couple px).
    // The bar is positioned one rAF after the sidebar effect runs, so poll until
    // its inline geometry CONVERGES on the active item (not merely exists) —
    // asserting immediately after the route flip races the measurement.
    const geometry = () =>
      app.evaluate(() => {
        const bar = document.getElementById("cf-rail-ind");
        const item = document.querySelector("aside.rail nav .nv.active");
        if (!bar || !item) return null;
        return {
          opacity: Number(getComputedStyle(bar).opacity),
          topDiff: Math.abs(parseFloat(bar.style.top || "NaN") - item.offsetTop),
          heightDiff: Math.abs(parseFloat(bar.style.height || "NaN") - item.offsetHeight),
        };
      });
    for (const route of ["/budgets", "/goals"]) {
      await nav(app, route);
      await expect
        .poll(async () => {
          const g = await geometry();
          return g && g.opacity > 0.5 && g.topDiff <= 2 && g.heightDiff <= 2;
        }, { message: `indicator converges on the active ${route} item` })
        .toBe(true);
    }
    // The old per-item bar is gone: exactly one indicator element, no ::before.
    const beforeBar = await app.evaluate(() => {
      const item = document.querySelector("aside.rail nav .nv.active");
      return item ? getComputedStyle(item, "::before").content : "none";
    });
    expect(beforeBar === "none" || beforeBar === "normal", "no per-item ::before indicator").toBe(true);
  });

  test("disabled controls keep readable 60% emphasis", async ({ app }) => {
    const v = await app.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue("--disabled-opacity").trim(),
    );
    expect(v).toBe("0.6");
  });

  test("keyboard focus draws the 2px accent ring with 2px offset", async ({ app }) => {
    await app.keyboard.press("Tab"); // skip-link / first focusable
    await app.keyboard.press("Tab");
    const ring = await app.evaluate(() => {
      const el = document.activeElement;
      if (!el || el === document.body) return null;
      const cs = getComputedStyle(el);
      return { width: cs.outlineWidth, style: cs.outlineStyle, offset: cs.outlineOffset, color: cs.outlineColor };
    });
    expect(ring).not.toBeNull();
    expect(ring.style).toBe("solid");
    expect(ring.width).toBe("2px");
    expect(ring.offset).toBe("2px");
    // Not transparent: the focused ring is painted.
    expect(ring.color === "rgba(0, 0, 0, 0)" || ring.color === "transparent").toBe(false);
  });
});
