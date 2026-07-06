// coverage.spec.mjs — the interactive-element coverage RATCHET. For every route
// it harvests the set of interactive controls (normalizing per-row dynamic ids to
// a stable pattern) and diffs against a committed manifest. A new or removed
// control fails the test until the manifest is intentionally regenerated — so the
// inventory of "what's clickable" can never drift silently, and a new page/control
// can't ship without a human acknowledging it.
//
// Regenerate after an intentional UI change:
//   UPDATE_COVERAGE=1 npx playwright test coverage.spec.mjs --project=chromium
//
// It also records, per route, the count of interactive elements that carry NO
// data-testid. That number is a ratchet: it may go down (as controls gain testids)
// but a rise fails the test — no new untestable controls slip in.
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect, ROUTES, nav } from "./fixtures.mjs";

const MANIFEST = path.join(path.dirname(fileURLToPath(import.meta.url)), "coverage-manifest.json");
const UPDATE = !!process.env.UPDATE_COVERAGE;

// harvestRoute runs in the browser: within #main (the page body, excluding the
// shared shell chrome), collect every interactive element, split into those with
// a normalized data-testid and a count of those without.
function harvestScript() {
  const SEL = [
    "button",
    "a[href]",
    "input:not([type=hidden])",
    "select",
    "textarea",
    "[role=button]",
    "[role=switch]",
    "[role=menuitem]",
    "[role=tab]",
    "[role=checkbox]",
    "[role=radio]",
    "[role=link]",
    "[contenteditable=true]",
  ].join(",");
  // Normalize a testid: collapse a trailing id-like segment (contains a digit, or
  // is a long alnum/hex token, or a uuid) to "*", so per-row ids like
  // "task-delete-btn-t_9f3a" become "task-delete-btn-*" — a stable pattern.
  const norm = (raw) => {
    let s = raw.replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, "*");
    const parts = s.split("-");
    const last = parts[parts.length - 1];
    if (last && (/\d/.test(last) || /^[0-9a-z]{8,}$/i.test(last) || /^[a-z]+_[0-9a-z]+$/i.test(last))) {
      parts[parts.length - 1] = "*";
      s = parts.join("-");
    }
    return s;
  };
  const main = document.querySelector("#main");
  if (!main) return { testids: [], untestided: 0 };
  const els = [...main.querySelectorAll(SEL)];
  const testids = new Set();
  let untestided = 0;
  for (const el of els) {
    const tid = el.getAttribute("data-testid");
    if (tid) testids.add(norm(tid));
    else untestided++;
  }
  return { testids: [...testids].sort(), untestided };
}

test("interactive-element coverage matches the committed manifest", async ({ app }) => {
  test.setTimeout(300_000);
  const live = {};
  for (const [route] of ROUTES) {
    await nav(app, route);
    live[route] = await app.evaluate(harvestScript);
  }

  if (UPDATE) {
    writeFileSync(MANIFEST, JSON.stringify(live, null, 2) + "\n");
    test.info().annotations.push({ type: "coverage", description: "manifest regenerated" });
    return;
  }

  let prev;
  try {
    prev = JSON.parse(readFileSync(MANIFEST, "utf8"));
  } catch {
    throw new Error("coverage-manifest.json missing — run with UPDATE_COVERAGE=1 to generate it");
  }

  const problems = [];
  for (const [route] of ROUTES) {
    const now = live[route];
    const was = prev[route];
    if (!was) {
      problems.push(`${route}: NEW route not in manifest — regenerate to acknowledge coverage`);
      continue;
    }
    const added = now.testids.filter((t) => !was.testids.includes(t));
    const removed = was.testids.filter((t) => !now.testids.includes(t));
    if (added.length) problems.push(`${route}: new control(s) ${JSON.stringify(added)} — add a test + regenerate`);
    if (removed.length) problems.push(`${route}: control(s) gone ${JSON.stringify(removed)} — removed on purpose? regenerate`);
    if (now.untestided > was.untestided) {
      problems.push(`${route}: untestable controls rose ${was.untestided}→${now.untestided} — give new controls a data-testid`);
    }
  }
  expect(problems, `coverage drift:\n${problems.join("\n")}`).toEqual([]);
});
