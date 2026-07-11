// scoring.mjs — Lighthouse-parity scoring for the CashFlux per-page performance
// framework. Every user-perceived metric (lower is better) is mapped to a 0..1
// sub-score through the SAME log-normal curve Lighthouse uses, then combined with
// weights into a 0..100 page score and a letter grade. This is deliberately
// "think Lighthouse, not code-level": it scores what a user feels (mount latency,
// blocking, layout stability, DOM weight), not Go function timings.
//
// The curve: a metric's control points are {p10, median}. p10 is the "good"
// threshold (maps to sub-score 0.90) and median maps to 0.50 — identical to
// Lighthouse's VRT curves. Worse (larger) values decay toward 0.

// erf/erfc via Abramowitz & Stegun 7.1.26 (max error ~1.5e-7) — enough for scoring.
function erf(x) {
  const sign = x < 0 ? -1 : 1;
  x = Math.abs(x);
  const t = 1 / (1 + 0.3275911 * x);
  const y =
    1 -
    ((((1.061405429 * t - 1.453152027) * t + 1.421413741) * t - 0.284496736) * t +
      0.254829592) *
      t *
      Math.exp(-x * x);
  return sign * y;
}
const erfc = (x) => 1 - erf(x);

// Solves erfc(z) = 1.8 → z ≈ -0.9061938; the multiplier that makes score(p10)=0.9.
const INVERSE_ERFC_ONE_FIFTH = 0.9061938024368232;

// logNormalScore returns a 0..1 sub-score for a lower-is-better metric, matching
// Lighthouse's getLogNormalScore(): score(median)=0.5, score(p10)=0.9.
export function logNormalScore({ p10, median }, value) {
  if (value === null || value === undefined || Number.isNaN(value)) return null;
  if (value <= 0) return 1;
  if (median <= 0 || p10 <= 0 || p10 >= median) {
    throw new Error(`bad control points p10=${p10} median=${median} (need 0<p10<median)`);
  }
  // 0 at value=median, +1 at value=p10 (p10<median so denominator<0).
  const standardizedX = (Math.log(value) - Math.log(median)) / (Math.log(p10) - Math.log(median));
  return clamp01(erfc(-standardizedX * INVERSE_ERFC_ONE_FIFTH) / 2);
}

const clamp01 = (n) => Math.max(0, Math.min(1, n));

// Control points per metric. p10 = good (green), median = midpoint (score 0.5).
// PAGE = warm SPA route transition (no wasm reload); LOAD = one-time cold boot.
export const PAGE_METRICS = {
  mountMs: { p10: 120, median: 450, weight: 25, label: "Route mount", unit: "ms",
    hint: "Time from navigation to the route's body appearing — the SPA analog of FCP." },
  tbtMs: { p10: 80, median: 350, weight: 30, label: "Blocking time", unit: "ms",
    hint: "Total blocking time from long tasks (>50ms) during the transition — main-thread jank." },
  stableMs: { p10: 220, median: 800, weight: 15, label: "Settle time", unit: "ms",
    hint: "Time until the page is visually stable (fonts + images + two frames) — the SPA analog of LCP/TTI." },
  cls: { p10: 0.02, median: 0.15, weight: 15, label: "Layout shift", unit: "",
    hint: "Cumulative layout shift during the transition — content jumping as it renders." },
  domNodes: { p10: 800, median: 3000, weight: 15, label: "DOM size", unit: "nodes",
    hint: "Interactive + element nodes in the page body — render weight the browser must lay out." },
};

export const LOAD_METRICS = {
  appReadyMs: { p10: 1500, median: 4000, weight: 30, label: "Time to interactive", unit: "ms",
    hint: "Cold boot: wasm download + instantiate + seed + first mount, until the app signals ready." },
  fcpMs: { p10: 1000, median: 2500, weight: 15, label: "First Contentful Paint", unit: "ms",
    hint: "When the first pixels of content paint." },
  lcpMs: { p10: 1200, median: 3000, weight: 20, label: "Largest Contentful Paint", unit: "ms",
    hint: "When the largest content element paints." },
  tbtMs: { p10: 200, median: 600, weight: 20, label: "Blocking time (boot)", unit: "ms",
    hint: "Total blocking time from long tasks during boot — wasm instantiate/seed jank." },
  transferMB: { p10: 2, median: 8, weight: 15, label: "Transfer weight", unit: "MB",
    hint: "Total bytes over the wire on cold load — dominated by the wasm binary." },
};

// scoreGroup computes weighted 0..100 from a metrics-definition map + measured values.
// Metrics measured as null are dropped and their weight redistributed (Lighthouse-style).
export function scoreGroup(defs, values) {
  let wsum = 0;
  let acc = 0;
  const parts = {};
  for (const [key, def] of Object.entries(defs)) {
    const sub = logNormalScore({ p10: def.p10, median: def.median }, values[key]);
    parts[key] = { value: values[key], sub, weight: def.weight, ...def };
    if (sub === null) continue;
    acc += sub * def.weight;
    wsum += def.weight;
  }
  const score = wsum > 0 ? Math.round((acc / wsum) * 100) : null;
  return { score, parts };
}

// Lighthouse's tri-color banding + a letter grade for at-a-glance ranking.
export function grade(score) {
  if (score === null) return { letter: "—", band: "n/a", color: "gray" };
  if (score >= 90) return { letter: score >= 97 ? "A+" : "A", band: "good", color: "green" };
  if (score >= 75) return { letter: "B", band: "good", color: "green" };
  if (score >= 60) return { letter: "C", band: "average", color: "orange" };
  if (score >= 45) return { letter: "D", band: "average", color: "orange" };
  return { letter: "F", band: "poor", color: "red" };
}

// ratingWord maps a sub-score to a Lighthouse-style word for prose analysis.
export function ratingWord(sub) {
  if (sub === null) return "not measured";
  if (sub >= 0.9) return "good";
  if (sub >= 0.5) return "needs improvement";
  return "poor";
}
