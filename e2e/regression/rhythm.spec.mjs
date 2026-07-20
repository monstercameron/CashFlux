// rhythm.spec.mjs — the unified "Bills & recurring" surface (the month's rhythm
// page) served identically on /recurring, /bills and /subscriptions.
//
// This suite replaces the three-tab era's scattered coverage. Every assertion
// here pins a behavior that was specified, or a bug that was actually fixed, on
// the redesigned surface:
//
//   - one surface on three routes (no tab control survives);
//   - the tideline hero's calm/flagged pinch and its stats agreeing with the
//     roster's own arithmetic;
//   - the overdue strip existing only when something IS overdue, its headline
//     agreeing with its rows, and Mark-paid actually emptying it (the strip and
//     the calendar now share ONE settled predicate — rhySettled — after they
//     disagreed about the same bill);
//   - the review strip's three counts agreeing, paging that does NOT bounce you
//     to page 1 on confirm/reject, and "Not recurring" having a way back;
//   - discovery quality: nothing already tracked, no habitual spend, cleaned
//     merchant names, and an evidence sentence on every candidate;
//   - the agenda's month headings (a monthly bill legitimately appears in more
//     than one month and must never appear twice in one);
//   - the calendar telling the truth about days that have gone by — and its
//     missed set being EXACTLY the overdue strip's set;
//   - the roster's ordering, lenses, and the formula-variable slug the engine
//     actually exposes.
//
// Clock note: the suite pins FIXED_NOW (2026-07-01). Two behaviors only exist
// once seeded due dates have gone by, so those tests boot at a later instant via
// bootAt and say so — asserting against the drifting real clock would be worse.
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect, nav, mainText, bootAt } from "./fixtures.mjs";

// shot resolves a screenshot name into e2e/, per repo convention, independent of
// the directory the runner happens to be invoked from.
const shot = (name) => path.join(path.dirname(fileURLToPath(import.meta.url)), "..", name);

// The three routes that all render the one surface.
const SURFACE_ROUTES = ["/recurring", "/bills", "/subscriptions"];

// A clock at which the seeded July commitments have gone by unpaid, so the
// overdue strip and the calendar's missed state both have something real to say.
const OVERDUE_NOW = "2026-07-22T12:00:00.000Z";

// This surface defers its review strip and roster past first paint (useAfterSettle
// keeps the discovery pipeline off the mount path), so "mounted" is not the same as
// "readable" here. The wait lives in fixtures' nav, which is generic over the
// data-settled flag, so nothing in this file has to think about it.

// money parses the app's accounting format — "($1,480.00)" is negative,
// "$4,700.00" positive — into minor units, so test arithmetic stays integral.
function money(text) {
  const m = String(text).match(/\(?\$([\d,]+\.\d{2})\)?/);
  if (!m) return null;
  const minor = Math.round(parseFloat(m[1].replace(/,/g, "")) * 100);
  return /^\(/.test(m[0]) ? -minor : minor;
}

// rosterRows reads the lineup's MAIN list (the collapsed "watching after
// cancellation" tail is a separate list with no testid) as {name, per, anchor}.
const rosterRows = (page) => readClaims(page, '[data-testid="rhy-roster"] .rhy-claim');

// allClaims includes the watching tail — the hero's monthly totals count every
// commitment, tail included, so the arithmetic check must too.
const allClaims = (page) => readClaims(page, "#main .rhy-claim");

// expandAgenda opens the real "show all" expander when the window is capped, so
// month grouping is asserted over the WHOLE horizon and not just the first page.
async function expandAgenda(page) {
  const all = page.getByTestId("rhy-agenda-showall");
  if (await all.count()) await all.click();
}

const readClaims = (page, sel) =>
  page.evaluate(
    (s) =>
      [...document.querySelectorAll(s)].map((row) => ({
        name: row.querySelector(".rhy-claim-name").innerText.trim(),
        per: row.querySelector(".rhy-claim-per").innerText.trim(),
        anchor: row.querySelector(".rhy-chip.is-anchor")?.innerText.trim() || null,
      })),
    sel,
  );

test.describe("unified surface", () => {
  for (const route of SURFACE_ROUTES) {
    test(`${route} renders the one Bills & recurring surface`, async ({ app }) => {
      await nav(app, route);
      // The hero title is the surface's identity — all three routes get it.
      await expect(app.locator("#main .rhy-sec-title").first()).toHaveText(/this month's rhythm/i);
      // Hero band: the tideline, or its honest empty state.
      await expect(
        app.locator('[data-testid="rhy-tideline"], [data-testid="rhy-tideline-empty"]'),
      ).toHaveCount(1);
      // The agenda and the roster sections are both present.
      await expect(app.locator("#sec-agenda")).toBeVisible();
      await expect(app.locator("#sec-roster")).toBeVisible();
      await expect(
        app.locator('[data-testid="rhy-roster"], [data-testid="rhy-roster-none"]'),
      ).toHaveCount(1);
    });
  }

  test("the Scheduled | Bills | Subscriptions tab control is gone", async ({ app }) => {
    await nav(app, "/recurring");
    // The old hub was a uiw.Segmented tablist. Nothing on the surface may be a
    // tab any more — the money lenses that inherited the old testids are
    // toggle buttons inside the roster's lens row, not a page-level tab strip.
    await expect(app.locator("#main [role='tab']")).toHaveCount(0);
    await expect(app.locator("#main [role='tablist']")).toHaveCount(0);
    for (const id of ["recurring-tab-scheduled", "recurring-tab-bills", "recurring-tab-subscriptions"]) {
      const btn = app.getByTestId(id);
      await expect(btn).toHaveAttribute("aria-pressed", /true|false/);
      await expect(btn.locator("xpath=ancestor::*[contains(@class,'rhy-lenses')]")).toHaveCount(1);
    }
  });

  test("/subscriptions lands with the Subscriptions lens preselected", async ({ app }) => {
    await nav(app, "/recurring");
    await expect(app.getByTestId("recurring-tab-scheduled")).toHaveAttribute("aria-pressed", "true");
    await nav(app, "/subscriptions");
    await expect(app.getByTestId("recurring-tab-subscriptions")).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("recurring-tab-scheduled")).toHaveAttribute("aria-pressed", "false");
  });
});

test.describe("tideline hero", () => {
  test("the band renders with an income up-tick and a today marker", async ({ app }) => {
    await nav(app, "/recurring");
    const tide = app.getByTestId("rhy-tideline");
    await expect(tide).toBeVisible();
    // Income is what makes it a PAY CYCLE — an up-tick must be drawn, not just
    // outflow ticks. (.rhy-tick-in is the income variant.)
    await expect(app.locator("#main .rhy-tick-in")).not.toHaveCount(0);
    await expect(app.locator("#main .rhy-tick-out")).not.toHaveCount(0);
    await expect(tide).toHaveAttribute("aria-label", /cushion/i);
  });

  test("the pinch reads calm on the sample data and still states the low point", async ({ app }) => {
    await nav(app, "/recurring");
    // RED IS RESERVED. The seeded household is not tight this cycle, so the hero
    // must say so calmly — and a calm state that reports nothing is decoration,
    // so it still names the lowest point and when it falls.
    await expect(app.getByTestId("rhy-pinch")).toHaveCount(0);
    const calm = app.getByTestId("rhy-pinch-calm");
    await expect(calm).toBeVisible();
    await expect(calm).toContainText(/no tight spots this cycle/i);
    await expect(calm).toContainText(/\$[\d,]+\.\d{2}/);
    await expect(calm).toContainText(/on [A-Z][a-z]{2} \d{1,2}/);
  });

  test("the hero stats are the roster's own arithmetic", async ({ app }) => {
    await nav(app, "/recurring");
    // net / in / out are claims about the same commitments the lineup lists. If
    // the two disagree the page is arguing with itself, so derive the totals from
    // the rendered roster rows and require an exact match.
    const rows = await allClaims(app);
    expect(rows.length).toBeGreaterThan(0);
    let inMinor = 0;
    let outMinor = 0;
    for (const r of rows) {
      const v = money(r.per);
      expect(v, `roster row "${r.name}" states a /mo figure`).not.toBeNull();
      if (v >= 0) inMinor += v;
      else outMinor += -v;
    }
    const stats = await app.evaluate(() =>
      Object.fromEntries(
        [...document.querySelectorAll("#main .rhy-stat")].map((s) => [
          s.querySelector(".rhy-stat-label").innerText.trim().toLowerCase(),
          s.querySelector(".rhy-stat-value").innerText.trim(),
        ]),
      ),
    );
    expect(money(stats["in / mo"])).toBe(inMinor);
    expect(money(stats["out / mo"])).toBe(outMinor);
    expect(money(stats["net / mo"])).toBe(inMinor - outMinor);
  });
});

test.describe("overdue strip", () => {
  test("it is absent when nothing is overdue", async ({ app }) => {
    // At FIXED_NOW nothing seeded has gone by unpaid. The strip is honest about
    // that by not existing — never an empty "0 overdue" band.
    await nav(app, "/recurring");
    await expect(app.getByTestId("rhy-overdue")).toHaveCount(0);
  });

  test("its count and total agree with the items it lists", async ({ page, errors }) => {
    void errors;
    // Clock moved forward: the seeded July 3/5 commitments have gone by.
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    const strip = page.getByTestId("rhy-overdue");
    await expect(strip).toBeVisible();
    const head = await strip.locator(".rhy-overdue-head").innerText();
    const rows = strip.locator('[data-testid^="rhy-overdue-row-"]');
    const n = await rows.count();
    expect(n).toBeGreaterThan(0);
    // "N items overdue · $X total" — both halves must describe the rows below.
    expect(Number(head.match(/(\d+) items? overdue/i)[1])).toBe(n);
    let sum = 0;
    for (let i = 0; i < n; i++) {
      sum += -money(await rows.nth(i).locator(".rhy-row-amt").innerText());
    }
    expect(money(head.match(/·\s*(\$[\d,]+\.\d{2})\s*total/i)[1])).toBe(sum);
  });

  test("Mark paid from the strip settles the item and it leaves the strip", async ({ page, errors }) => {
    void errors;
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    const strip = page.getByTestId("rhy-overdue");
    await expect(strip).toBeVisible();
    const before = await strip.locator('[data-testid^="rhy-overdue-row-"]').count();
    const firstRow = strip.locator('[data-testid^="rhy-overdue-row-"]').first();
    const id = (await firstRow.getAttribute("data-testid")).replace("rhy-overdue-row-", "");
    const name = (await firstRow.locator(".rhy-row-name").innerText()).trim();
    // This was a real bug: the strip honoured only the bill-match link, so marking
    // an overdue item paid FROM THIS STRIP left it sitting in the strip. The strip
    // and the calendar now share one settled predicate (rhySettled).
    await page.getByTestId(`rhy-mark-paid-${id}`).click();
    await expect(page.getByTestId(`rhy-overdue-row-${id}`)).toHaveCount(0, { timeout: 20_000 });
    if (before > 1) {
      await expect(page.getByTestId("rhy-overdue")).toBeVisible();
      await expect(page.getByTestId("rhy-overdue")).not.toContainText(name);
    } else {
      await expect(page.getByTestId("rhy-overdue")).toHaveCount(0);
    }
  });
});

test.describe("review strip", () => {
  test("the headline, the lane header and the pager all state the same total", async ({ app }) => {
    await nav(app, "/recurring");
    // Three figures used to disagree here — 57, 6 and 5 — of which only the last
    // was reachable. They now derive from ONE split (rhySplitCandidates).
    const title = await app.locator("#sec-review .rhy-sec-title").innerText();
    const headline = Number(title.match(/(\d+) to review/i)[1]);
    const lane = await app.locator("#sec-review .rhy-group-head").first().innerText();
    expect(Number(lane.match(/found (\d+) repeating charge/i)[1])).toBe(headline);
    const range = await app.getByTestId("rhy-review-range").innerText();
    expect(Number(range.match(/of (\d+)/)[1])).toBe(headline);
    // And the count is the number of candidate cards actually reachable.
    const shown = await app.locator("#sec-review .rhy-cand").count();
    expect(shown).toBeLessThanOrEqual(headline);
    expect(Number(range.match(/^(\d+)[–-](\d+)/)[2])).toBe(shown);
  });

  test("every candidate carries a non-empty evidence sentence", async ({ app }) => {
    await nav(app, "/recurring");
    const cards = app.locator("#sec-review .rhy-cand");
    const n = await cards.count();
    expect(n).toBeGreaterThan(0);
    for (let i = 0; i < n; i++) {
      const ev = (await cards.nth(i).locator(".rhy-cand-ev").innerText()).trim();
      // A candidate that cannot justify itself has nothing to show the user.
      expect(ev.length, `candidate ${i} evidence sentence`).toBeGreaterThan(0);
      expect(ev).toMatch(/\d+ payments/i);
      expect(ev).toMatch(/\$[\d,]+\.\d{2}/);
    }
  });

  test("candidate names are cleaned merchants, not raw bank descriptors", async ({ app }) => {
    await nav(app, "/recurring");
    const names = await app.evaluate(() =>
      [...document.querySelectorAll("#sec-review .rhy-cand-name")].map((e) => e.innerText.trim()),
    );
    expect(names.length).toBeGreaterThan(0);
    for (const n of names) {
      // No processor prefixes, reference numbers, or trailing phone numbers —
      // "Msft * Xbox Game Pass 425-6816830" must read as "Xbox Game Pass".
      expect(n, `candidate name "${n}"`).not.toMatch(/\*/);
      expect(n, `candidate name "${n}"`).not.toMatch(/\d{3}-\d{7}/);
      expect(n, `candidate name "${n}"`).not.toMatch(/\d{6,}/);
      expect(n, `candidate name "${n}"`).not.toMatch(/#/);
    }
    // The raw descriptor is not lost — it belongs in the evidence list, which is
    // what the user will recognise on a statement.
    const xbox = app.getByTestId("rhy-review-xbox-game-pass");
    if (await xbox.count()) {
      await app.getByTestId("rhy-review-evidence-xbox-game-pass").click();
      await expect(xbox.locator(".rhy-ev-list")).toBeVisible();
      await expect(xbox.locator(".rhy-ev-list")).toContainText(/msft/i);
    }
  });

  test("already-tracked commitments and habitual spend are never proposed", async ({ app }) => {
    await nav(app, "/recurring");
    const names = await app.evaluate(() =>
      [...document.querySelectorAll("#sec-review .rhy-cand-name")].map((e) => e.innerText.trim().toLowerCase()),
    );
    const tracked = (await rosterRows(app)).map((r) => r.name.toLowerCase());
    expect(tracked.length).toBeGreaterThan(0);
    for (const t of tracked) {
      // Dedupe is by settled signature, not label — a commitment the household
      // already plans is a CYCLE of that commitment, never a new candidate.
      expect(names, `"${t}" is already tracked and must not be a candidate`).not.toContain(t);
    }
    // Eating out, groceries and fuel repeat like clockwork and are not
    // commitments. They are demoted to the weak-signal lane, never proposed.
    for (const n of names) {
      expect(n, `candidate "${n}"`).not.toMatch(
        /coffee|cafe|café|starbucks|dining|restaurant|takeaway|grocer|supermarket|market basket|gas|fuel|shell|chevron/i,
      );
    }
  });

  test("'weaker signals' opens Detection preferences and lists them", async ({ app }) => {
    await nav(app, "/recurring");
    const link = app.getByTestId("rhy-weak-signals");
    await expect(link).toBeVisible();
    const claimed = Number((await link.innerText()).match(/(\d+) weaker signals/i)[1]);
    expect(claimed).toBeGreaterThan(0);
    await link.click();
    // A number the user is told about and cannot reach is worse than no number.
    const prefs = app.getByTestId("subs-detect-prefs");
    await expect(prefs).toBeVisible({ timeout: 20_000 });
    const list = app.getByTestId("rhy-weak-signals-list");
    await expect(list).toBeVisible();
    expect(await list.locator("li").count()).toBe(claimed);
    // Each demoted signal shows the same evidence the strip would have shown, so
    // "too weak" is a judgment the user can check.
    await expect(list.locator("li").first()).toContainText(/\d+ payments/i);
  });

  test("'Not recurring' is undoable — the hidden list gives it a way back", async ({ app }) => {
    await nav(app, "/recurring");
    const card = app.locator("#sec-review .rhy-cand").first();
    const testid = await card.getAttribute("data-testid");
    const slug = testid.replace("rhy-review-", "");
    const name = (await card.locator(".rhy-cand-name").innerText()).trim();
    const before = await app.locator("#sec-review .rhy-cand").count();

    await app.getByTestId(`rhy-review-reject-${slug}`).click();
    await expect(app.getByTestId(testid)).toHaveCount(0, { timeout: 20_000 });

    // A destructive single unconfirmed click needs an undo. It lives beside the
    // other detection judgments, in Detection preferences.
    await app.getByTestId("subs-detect-prefs-toggle").click();
    const hidden = app.getByTestId("rhy-hidden-list");
    await expect(hidden).toBeVisible({ timeout: 20_000 });
    await expect(hidden).toContainText(name);
    // Located by the name the user recognises, not by slug: the suppressed
    // SIGNATURE is the cluster's canonical bank form ("MSFT XBOX GAME PASS #"),
    // which is deliberately not the cleaned display name the card shows.
    const row = hidden.locator("li").filter({ hasText: name }).first();
    await row.getByRole("button").click();
    // The modal sees its own change (it subscribes to the data revision).
    await expect(hidden.locator("li").filter({ hasText: name })).toHaveCount(0, { timeout: 20_000 });
    await app.getByTestId("subs-prefs-done").click();
    await expect(app.getByTestId(testid)).toBeVisible({ timeout: 20_000 });
    await expect(app.locator("#sec-review .rhy-cand")).toHaveCount(before);
  });

  test("confirming a candidate adds it to the lineup and back-claims its history", async ({ app }) => {
    await nav(app, "/recurring");
    const card = app.locator("#sec-review .rhy-cand").first();
    const slug = (await card.getAttribute("data-testid")).replace("rhy-review-", "");
    const name = (await card.locator(".rhy-cand-name").innerText()).trim();
    await app.getByTestId(`rhy-review-confirm-${slug}`).click();
    // It joins the plan…
    await expect(app.locator('[data-testid="rhy-roster"]')).toContainText(name, { timeout: 20_000 });
    // …and stops being a candidate (dedupe now recognises it as tracked).
    await expect(app.getByTestId(`rhy-review-${slug}`)).toHaveCount(0);
  });

  test("with no OpenAI key the Smart+ footer is one coherent disabled state", async ({ app }) => {
    await nav(app, "/recurring");
    const optin = app.getByTestId("rhy-smartplus-optin");
    await expect(optin).toBeVisible();
    // Never an enabled-looking button beside a "you need a key" note: the button
    // says what it would do, is really disabled, and the sentence says why and
    // where to fix it.
    await expect(optin).toBeDisabled();
    await expect(optin).toHaveAttribute("aria-disabled", "true");
    const foot = app.locator("#sec-review .rhy-review-foot");
    await expect(foot).toHaveClass(/is-disabled/);
    await expect(foot).toContainText(/needs your own openai key/i);
    await expect(foot).toContainText(/tokens/i);
    await expect(foot.locator("a.link")).toHaveAttribute("href", /\/settings/);
    // The disabled label must not also quote the invitation copy.
    await expect(optin).not.toHaveText(/on your openai key/i);
  });
});

test.describe("review strip paging", () => {
  // The seeded dataset yields 3 Smart candidates — under one page — so the paging
  // contract is exercised on the Smart+ lane, which pages through the SAME
  // pagination.Clamp/Bounds code path with 50+ leftovers. A key is configured so
  // the lane can open; the model call itself is blocked (the lane re-scores every
  // row locally regardless of what the model says, which is the point).
  async function openDeeperLane(page) {
    await page.route(/api\.openai\.com/, (r) => r.abort());
    await nav(page, "/settings");
    await page.locator(".settings-page .set-tab-strip button", { hasText: /^AI/i }).first().click();
    const key = page.locator('.settings-page input.set-input[type="password"]').first();
    await expect(key).toBeVisible();
    await key.fill("sk-e2e-not-a-real-key");
    await nav(page, "/recurring");
    const optin = page.getByTestId("rhy-smartplus-optin");
    await expect(optin).toBeEnabled({ timeout: 20_000 });
    await optin.click();
    await expect(page.getByTestId("rhy-review-plus-range")).toBeVisible({ timeout: 30_000 });
  }

  test("paging works and rejecting does NOT bounce back to page 1", async ({ app }) => {
    await openDeeperLane(app);
    const range = app.getByTestId("rhy-review-plus-range");
    const total = Number((await range.innerText()).match(/of (\d+)/)[1]);
    test.skip(total <= 5, "needs more than one page of leftovers");

    await expect(range).toHaveText(/^1[–-]\d+ of/);
    await app.getByTestId("rhy-review-plus-next").click();
    await expect(range).toHaveText(/^6[–-]\d+ of/, { timeout: 20_000 });

    // Reject a candidate ON PAGE 2. Confirm/reject must not reset the page —
    // being thrown back to page 1 after every judgment makes a 50-item queue
    // unworkable.
    const cards = app.locator(".rhy-review-group").last().locator(".rhy-cand");
    const slug = (await cards.first().getAttribute("data-testid")).replace("rhy-review-", "");
    await app.getByTestId(`rhy-review-reject-${slug}`).click();
    await expect(app.getByTestId(`rhy-review-${slug}`)).toHaveCount(0, { timeout: 20_000 });
    // Still on page 2 (one fewer item overall).
    await expect(range).toHaveText(new RegExp(`^6[–-]\\d+ of ${total - 1}$`), { timeout: 20_000 });
  });

  test("emptying the last page falls back one page instead of to page 1", async ({ app }) => {
    await openDeeperLane(app);
    const range = app.getByTestId("rhy-review-plus-range");
    const total = Number((await range.innerText()).match(/of (\d+)/)[1]);
    test.skip(total <= 25, "needs a partial last page at the 25 page size");

    // 25 per page makes the last page short, so it can be emptied in a few clicks.
    await app.locator(".rhy-review-group").last().getByRole("button", { name: "25", exact: true }).click();
    const lastPage = Math.ceil(total / 25);
    const jump = app.getByTestId("rhy-review-plus-jump");
    await jump.fill(String(lastPage));
    await jump.press("Enter");
    await expect(range).toHaveText(new RegExp(`^${(lastPage - 1) * 25 + 1}[–-]`), { timeout: 20_000 });

    const onLast = total - (lastPage - 1) * 25;
    for (let i = 0; i < onLast; i++) {
      const card = app.locator(".rhy-review-group").last().locator(".rhy-cand").first();
      const slug = (await card.getAttribute("data-testid")).replace("rhy-review-", "");
      await app.getByTestId(`rhy-review-reject-${slug}`).click();
      await expect(app.getByTestId(`rhy-review-${slug}`)).toHaveCount(0, { timeout: 20_000 });
    }
    // Clamped back exactly ONE page, not to page 1.
    await expect(range).toHaveText(new RegExp(`^${(lastPage - 2) * 25 + 1}[–-]`), { timeout: 20_000 });
  });
});

test.describe("up next — agenda", () => {
  test("the window is grouped by month and a monthly bill appears once per month", async ({ app }) => {
    await nav(app, "/recurring");
    await expandAgenda(app);
    const months = await app.evaluate(() =>
      [...document.querySelectorAll("#sec-agenda .rhy-ag-month")].map((e) => e.innerText.trim()),
    );
    // The 45-day horizon crosses a month boundary, so there is more than one
    // heading and none repeats.
    expect(months.length).toBeGreaterThan(1);
    expect(new Set(months).size).toBe(months.length);

    // Undivided, a monthly bill listed twice reads as owing it twice. Group the
    // rows under their heading and require every name to be unique WITHIN a month
    // while at least one recurs ACROSS months.
    const byMonth = await app.evaluate(() => {
      const out = {};
      let cur = null;
      for (const el of document.querySelectorAll("#sec-agenda .rhy-ag-month, #sec-agenda .rhy-ag-row")) {
        if (el.classList.contains("rhy-ag-month")) {
          cur = el.innerText.trim();
          out[cur] = [];
        } else if (cur) {
          out[cur].push(el.querySelector(".rhy-ag-name").innerText.trim());
        }
      }
      return out;
    });
    for (const [m, names] of Object.entries(byMonth)) {
      expect(new Set(names).size, `"${m}" lists each commitment once`).toBe(names.length);
    }
    const all = Object.values(byMonth).flat();
    expect(all.length, "a monthly commitment recurs across months").toBeGreaterThan(
      new Set(all).size,
    );
  });

  test("the section note states the window it actually draws", async ({ app }) => {
    await nav(app, "/recurring");
    // The compact list runs the whole 45-day horizon — saying "this pay cycle"
    // while listing September was the page overpromising its own scope.
    await expect(app.locator("#sec-agenda .rhy-sec-note")).toContainText(/next 45 days/i);
    await app.getByTestId("rhy-view-calendar").click();
    await expect(app.locator("#sec-agenda .rhy-sec-note").first()).toContainText(/a month at a time/i);
  });

  test("the COMPACT | CALENDAR choice persists across a reload", async ({ app }) => {
    // The only test here that reboots the app: a full reload re-downloads and
    // re-instantiates the wasm and re-seeds the store, which alone can eat most of
    // the default 60s budget on a loaded machine — this test was already the
    // suite's flakiest for that reason. Landing on the surface twice, each landing
    // now waiting out the deferred sections, is enough to tip it over. The budget is
    // the thing that is wrong here, not the behavior, so the budget is what moves.
    test.setTimeout(150_000);
    await nav(app, "/recurring");
    await expect(app.getByTestId("rhy-view-compact")).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("rhy-agenda")).toBeVisible();

    await app.getByTestId("rhy-view-calendar").click();
    await expect(app.getByTestId("rhy-view-calendar")).toHaveAttribute("aria-pressed", "true");
    await expect(app.locator("#main .rhy-cal")).toBeVisible();
    await expect(app.getByTestId("rhy-agenda")).toHaveCount(0);

    // Persisted preference, not component state.
    //
    // Let the write become DURABLE before reloading. SettingKVSet lands in the
    // in-memory dataset immediately but reaches IndexedDB via a 250ms debounced
    // persist, so a reload inside that window legitimately loses the preference.
    // This is a real product race, not a test artifact — it is only reachable by a
    // user who toggles and reloads within a quarter-second, which is why the test
    // used to pass: the page was slow enough that the assertions above spent longer
    // than the debounce. Making the page fast removed that accidental cushion.
    // Tracked as RH-PERSIST1; until it is fixed the test waits rather than
    // pretending the preference is durable the instant it is set.
    await app.waitForTimeout(800);
    await app.reload();
    await app.waitForFunction(
      () => document.documentElement.getAttribute("data-app-ready") === "true",
      null,
      { timeout: 45_000 },
    );
    await nav(app, "/recurring");
    await expect(app.getByTestId("rhy-view-calendar")).toHaveAttribute("aria-pressed", "true", {
      timeout: 20_000,
    });
    await expect(app.locator("#main .rhy-cal")).toBeVisible();

    // Put it back so the persisted pref doesn't leak into another expectation.
    await app.getByTestId("rhy-view-compact").click();
    await expect(app.getByTestId("rhy-agenda")).toBeVisible();
  });

  for (const width of [1440, 1202, 900]) {
    test(`the date column never collides with the name @${width}`, async ({ app }) => {
      await app.setViewportSize({ width, height: 1100 });
      await nav(app, "/recurring");
      await expect(app.getByTestId("rhy-agenda")).toBeVisible();
      const gaps = await app.evaluate(() =>
        [...document.querySelectorAll("#sec-agenda .rhy-ag-row")].map((row) => {
          const d = row.querySelector(".rhy-ag-date").getBoundingClientRect();
          const n = row.querySelector(".rhy-ag-name").getBoundingClientRect();
          return { gap: Math.round(n.left - d.right), date: row.querySelector(".rhy-ag-date").innerText };
        }),
      );
      expect(gaps.length).toBeGreaterThan(0);
      for (const g of gaps) {
        // A POSITIVE gap: the column is sized for the dates it really renders.
        expect(g.gap, `date "${g.date}" clears the name`).toBeGreaterThan(0);
      }
    });
  }
});

test.describe("agenda — calendar view", () => {
  test("day cells carry names AND amounts, not bare dots", async ({ app }) => {
    await nav(app, "/recurring");
    await app.getByTestId("rhy-view-calendar").click();
    const items = app.locator("#main .rhy-cal-item");
    await expect(items.first()).toBeVisible();
    const n = await items.count();
    expect(n).toBeGreaterThan(0);
    for (let i = 0; i < n; i++) {
      await expect(items.nth(i).locator(".rhy-cal-name")).not.toBeEmpty();
      await expect(items.nth(i).locator(".rhy-cal-amt")).toContainText(/\$[\d,]+\.\d{2}/);
    }
  });

  test("prev / next page across a month boundary and This month returns", async ({ app }) => {
    await nav(app, "/recurring");
    await app.getByTestId("rhy-view-calendar").click();
    const heading = app.locator("#sec-agenda .rhy-sec-note").last();
    const start = (await heading.innerText()).trim();
    await app.getByTestId("cal-next").click();
    await expect(heading).not.toHaveText(start, { timeout: 20_000 });
    const next = (await heading.innerText()).trim();
    await app.getByTestId("cal-prev").click();
    await expect(heading).toHaveText(start, { timeout: 20_000 });
    expect(next).not.toBe(start);
    // cal-today only exists once you have paged away from this month.
    await app.getByTestId("cal-next").click();
    await expect(app.getByTestId("cal-today")).toBeVisible();
    await app.getByTestId("cal-today").click();
    await expect(app.getByTestId("cal-today")).toHaveCount(0);
  });

  test("past days show settled, missed and nothing-known as three distinct states", async ({ page, errors }) => {
    void errors;
    // Needs a month with days already gone by AND something unpaid in them.
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    await page.getByTestId("rhy-view-calendar").click();
    await expect(page.locator("#main .rhy-cal")).toBeVisible();
    const done = page.locator("#main .rhy-cal-item.is-done");
    const missed = page.locator("#main .rhy-cal-item.is-missed");
    const past = page.locator("#main .rhy-cal-item.is-past");
    await expect(done.first()).toBeVisible();
    await expect(missed.first()).toBeVisible();
    await expect(past.first()).toBeVisible();
    // Visually distinct — three different claims must not render identically.
    const style = (loc) =>
      loc.first().evaluate((el) => {
        const s = getComputedStyle(el);
        return [s.color, s.backgroundColor, s.borderColor, s.opacity, s.textDecorationLine].join("|");
      });
    const [a, b, c] = [await style(done), await style(missed), await style(past)];
    expect(a).not.toBe(b);
    expect(b).not.toBe(c);
    expect(a).not.toBe(c);
  });

  test("the calendar's missed set is EXACTLY the overdue strip's items", async ({ page, errors }) => {
    void errors;
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    // Trust invariant: the calendar must never accuse the user of missing a bill
    // the strip does not count. The strip previously said three items were
    // overdue while the calendar painted a fourth in the same warning tone, and a
    // user cannot tell which of two disagreeing claims to believe.
    const stripNames = await page.evaluate(() =>
      [...document.querySelectorAll('[data-testid^="rhy-overdue-row-"] .rhy-row-name')]
        .map((e) => e.innerText.trim())
        .sort(),
    );
    expect(stripNames.length).toBeGreaterThan(0);
    await page.getByTestId("rhy-view-calendar").click();
    await expect(page.locator("#main .rhy-cal")).toBeVisible();
    const missedNames = await page.evaluate(() =>
      [...new Set(
        [...document.querySelectorAll("#main .rhy-cal-item.is-missed .rhy-cal-name")].map((e) =>
          e.innerText.trim(),
        ),
      )].sort(),
    );
    expect(missedNames).toEqual(stripNames);
  });
});

test.describe("the lineup — roster", () => {
  test("the default order is largest per-month first", async ({ app }) => {
    await nav(app, "/recurring");
    const rows = await rosterRows(app);
    const weights = rows.map((r) => Math.abs(money(r.per)));
    expect(weights.length).toBeGreaterThan(1);
    for (let i = 1; i < weights.length; i++) {
      expect(weights[i], `row ${i} ("${rows[i].name}") is no heavier than the one above`).toBeLessThanOrEqual(
        weights[i - 1],
      );
    }
    await expect(app.getByTestId("rhy-sort")).toHaveValue("size");
  });

  test("the sort picker really reorders the lineup", async ({ app }) => {
    await nav(app, "/recurring");
    const before = (await rosterRows(app)).map((r) => r.name);
    await app.getByTestId("rhy-sort").selectOption("name");
    await expect(app.getByTestId("rhy-sort")).toHaveValue("name");
    const byName = (await rosterRows(app)).map((r) => r.name);
    expect(byName).not.toEqual(before);
    expect(byName).toEqual([...byName].sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase())));
  });

  test("the money lenses filter to what they name", async ({ app }) => {
    await nav(app, "/recurring");
    const all = await rosterRows(app);
    expect(all.length).toBeGreaterThan(1);

    // Income: only inflows.
    await app.getByTestId("rhy-lens-income").click();
    const income = await rosterRows(app);
    expect(income.length).toBeGreaterThan(0);
    expect(income.length).toBeLessThan(all.length);
    for (const r of income) expect(money(r.per)).toBeGreaterThan(0);

    // Bills: account-tied outflows, each showing its anchor chip.
    await app.getByTestId("recurring-tab-bills").click();
    const billRows = await rosterRows(app);
    expect(billRows.length).toBeGreaterThan(0);
    for (const r of billRows) {
      expect(money(r.per), `"${r.name}" is an outflow`).toBeLessThan(0);
      expect(r.anchor, `"${r.name}" is account-tied`).not.toBeNull();
    }

    // All: back to everything.
    await app.getByTestId("recurring-tab-scheduled").click();
    expect((await rosterRows(app)).length).toBe(all.length);
  });

  test("the Subscriptions lens shows real subscriptions and their subtotal", async ({ app }) => {
    // RH3, fixed. The account anchor means "tied to a liability this payment
    // SETTLES", not the funding account it posts from — reading
    // domain.Recurring.AccountID (the funding account, which every seeded flow
    // carries) made "bills" swallow everything and left this lens unreachable.
    //
    // The model this now guards: Bills = liability-anchored
    // (bills.LiabilityAnchors, the same DedupeObligations pipeline the agenda
    // runs); Subscriptions is NOT the complement but a genuine lens over
    // free-floating subscription-ish commitments; anything that is neither (HOA
    // dues, property tax, insurance) appears under All only. Lenses are filters,
    // not a partition, and the subtotal counts only real subscriptions.
    await nav(app, "/subscriptions");
    await expect(app.getByTestId("recurring-tab-subscriptions")).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("rhy-roster-none")).toHaveCount(0);
    const subs = await rosterRows(app);
    expect(subs.length, "the Subscriptions lens matches at least one commitment").toBeGreaterThan(0);
    // Free-floating: a subscription is not anchored to a liability it settles.
    const subtotal = app.getByTestId("rhy-subs-subtotal");
    await expect(subtotal).toBeVisible();
    await expect(subtotal).toContainText(/\/ mo/);
    await expect(subtotal).toContainText(/\/ yr/);
    // The chip's own arithmetic: the monthly figure is the lens's own rows.
    const monthly = subs.reduce((n, r) => n + Math.abs(money(r.per)), 0);
    expect(money((await subtotal.innerText()).split("·")[0])).toBe(monthly);
  });

  test("an anchor chip links to its account", async ({ app }) => {
    await nav(app, "/recurring");
    const chip = app.locator("#main .rhy-chip.is-anchor").first();
    await expect(chip).toBeVisible();
    await chip.click();
    await expect(app.locator('#main[data-route="/accounts"]').first()).toBeVisible({ timeout: 20_000 });
  });

  test("the kebab holds the destructive action and the formula variable the engine exposes", async ({ app }) => {
    await nav(app, "/recurring");
    // Delete is destructive, so it lives ONLY behind the ⋯ — never on the row face.
    await expect(app.locator('#main .rhy-claim [data-testid^="recurring-del-"]:visible')).toHaveCount(0);
    await app.getByTestId("recurring-menu-rec-gym").click();
    const menu = app.locator(".add-menu:not(.hidden-menu)").first();
    await expect(menu.locator('[data-testid="recurring-del-rec-gym"]')).toBeVisible();
    await expect(menu.locator('[data-testid="recurring-del-rec-gym"]')).toHaveClass(/danger/);

    // Slug trap: engineenv.RecurringVarBases assigns prefixes in the order the
    // flows are PASSED (store order), and computeRecurView slugs before the
    // display sort — so the name offered here is the one the formula surface
    // exposes. The engine side of this exact string is locked natively by
    // internal/engineenv/recurringvars_surface_test.go.
    await menu.locator('[data-testid="rhy-copyvar-rec-gym"]').click();
    await expect(app.locator("body")).toContainText("recurring_gym_membership_monthly", {
      timeout: 20_000,
    });
  });
});

test.describe("keyboard reachability", () => {
  test("the primary verbs are real, focusable, keyboard-operable controls", async ({ app }) => {
    await nav(app, "/recurring");
    const verbs = [
      "rhy-view-compact",
      "rhy-view-calendar",
      "recurring-tab-scheduled",
      "recurring-tab-bills",
      "recurring-tab-subscriptions",
      "rhy-lens-income",
      "rhy-review-prev",
      "rhy-review-next",
      "rhy-weak-signals",
    ];
    for (const id of verbs) {
      const el = app.getByTestId(id);
      await expect(el, `${id} exists`).toHaveCount(1);
      const tag = await el.evaluate((e) => e.tagName.toLowerCase());
      expect(tag, `${id} is a real control`).toBe("button");
      // A disabled control correctly refuses focus (the pager's Prev is disabled
      // on page 1) — reachability only means the ENABLED verbs take focus.
      if (!(await el.isEnabled())) continue;
      await el.focus();
      expect(
        await app.evaluate((t) => document.activeElement?.getAttribute("data-testid") === t, id),
        `${id} takes focus`,
      ).toBe(true);
    }
    // The lineup's sort is a labelled native select, reachable the same way.
    const sort = app.getByTestId("rhy-sort");
    await sort.focus();
    await expect(sort).toBeFocused();
    await expect(sort).toHaveAttribute("aria-label", /sort/i);
  });

  test("Confirm and Not recurring are operable from the keyboard alone", async ({ app }) => {
    await nav(app, "/recurring");
    const card = app.locator("#sec-review .rhy-cand").first();
    const slug = (await card.getAttribute("data-testid")).replace("rhy-review-", "");
    const reject = app.getByTestId(`rhy-review-reject-${slug}`);
    await expect(app.getByTestId(`rhy-review-confirm-${slug}`)).toBeEnabled();
    await reject.focus();
    await expect(reject).toBeFocused();
    await app.keyboard.press("Enter");
    await expect(app.getByTestId(`rhy-review-${slug}`)).toHaveCount(0, { timeout: 20_000 });
  });

  test("Mark paid on an overdue item is operable from the keyboard alone", async ({ page, errors }) => {
    void errors;
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    const row = page.locator('[data-testid^="rhy-overdue-row-"]').first();
    const id = (await row.getAttribute("data-testid")).replace("rhy-overdue-row-", "");
    const btn = page.getByTestId(`rhy-mark-paid-${id}`);
    await btn.focus();
    await expect(btn).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByTestId(`rhy-overdue-row-${id}`)).toHaveCount(0, { timeout: 20_000 });
  });
});

test.describe("migrated from the three-tab era", () => {
  test("a liability and its recurring flow are not double-counted", async ({ app }) => {
    // Was interactions.spec "bills: a liability + its recurring flow are not
    // double-counted". The invariant is now enforced by bills.DedupeObligations
    // and is asserted on the merged agenda, where both used to appear.
    await nav(app, "/bills");
    await expandAgenda(app);
    const names = await app.evaluate(() =>
      [...document.querySelectorAll("#sec-agenda .rhy-ag-row")].map((r) => ({
        name: r.querySelector(".rhy-ag-name").innerText.trim().toLowerCase(),
        date: r.querySelector(".rhy-ag-date").innerText.trim(),
      })),
    );
    const carRows = names.filter((n) => /car payment \(marcus\)/.test(n.name));
    // One obligation, one row per due date.
    expect(new Set(carRows.map((r) => r.date)).size).toBe(carRows.length);
    const text = await mainText(app);
    expect((text.match(/car payment \(marcus\)/gi) || []).length).toBeGreaterThan(0);
  });

  test("a merged obligation keeps the liability it settles visible", async ({ app }) => {
    // The other half of dedupe: folding the statement bill into the flow must not
    // lose the account identity, so the merged row carries its anchor chip.
    await nav(app, "/recurring");
    const row = app
      .locator("#sec-agenda .rhy-ag-row")
      .filter({ hasText: /car payment \(marcus\)/i })
      .first();
    await expect(row).toBeVisible();
    await expect(row.locator(".rhy-chip")).toContainText(/car loan/i);
  });

  test("planned recurring is never offered a cancellation helper", async ({ app }) => {
    // Was interactions.spec "subscriptions: share is sane and planned recurring
    // isn't flagged cancellable". Shares are now the roster's %-of-outflow spine.
    await nav(app, "/subscriptions");
    await expect(app.locator("body")).not.toContainText(/how to cancel hoa/i);
    await app.getByTestId("recurring-tab-scheduled").click();
    const pcts = await app.evaluate(() =>
      [...document.querySelectorAll("#main .rhy-spine-pct")].map((e) =>
        Number(e.innerText.replace(/[^\d.]/g, "")),
      ),
    );
    expect(pcts.length).toBeGreaterThan(0);
    for (const p of pcts) expect(p).toBeLessThanOrEqual(100);
  });

  test("a cancelled subscription that charges again raises a finding with a one-click to-do", async ({ app }) => {
    // Was gapfeatures.spec "subscription cancel helper: 'remind me' creates a
    // tracked cancellation task". The reminder verb became the findings strip's
    // charged-after-cancellation row, which is the honest version: it fires on
    // evidence rather than on demand.
    await nav(app, "/recurring");
    const dispute = app.getByTestId("rhy-finding-dispute");
    await expect(dispute).toBeVisible();
    const finding = app.locator("#main .rhy-finding").filter({ has: dispute }).first();
    await expect(finding).toContainText(/after you cancelled/i);
    await dispute.first().click();
    await nav(app, "/todo");
    await expect(app.locator("#main")).toContainText(/dispute/i, { timeout: 20_000 });
  });

  test("bill negotiation lives in the agenda row's kebab", async ({ app }) => {
    // Was gapfeatures.spec "bill negotiation helper" on /bills. The verb survived,
    // relocated from the old bill-menu-btn-* into the agenda row's ⋯.
    await nav(app, "/bills");
    await app.locator('[data-testid^="rhy-ag-menu-"]').first().click();
    const item = app.locator('.add-menu:not(.hidden-menu) [data-testid^="bill-negotiate-"]').first();
    await expect(item).toBeVisible();
    await item.click();
    // It still tracks the follow-up as a real to-do — via the composer, which is
    // where the talking points are handed over (RH5). The click opens the seeded
    // composer; submitting it is what files the task, so the flow is driven to
    // its end rather than asserting on a task the user never agreed to.
    await app.getByTestId("task-add-submit").click();
    await nav(app, "/todo");
    // The seeded household already has 45 tasks and the list pages at 20, so the
    // new one is not necessarily on page 1 — searched for rather than scrolled to,
    // which is what a user would do and what makes the assertion about the task
    // existing rather than about where it sorted.
    await app.getByTestId("todo-search").fill("Negotiate");
    await expect(app.locator("#main")).toContainText(/negotiat/i, { timeout: 20_000 });
  });

  test("bill negotiation hands the user the talking points", async ({ app }) => {
    // RH5, fixed. The haggling script is the whole feature and the to-do is just
    // the follow-up, so Negotiate seeds the task COMPOSER with
    // subscriptions.NegotiationTips rather than silently filing a bare
    // "Negotiate <name>" task the user has no idea how to act on.
    await nav(app, "/bills");
    await app.locator('[data-testid^="rhy-ag-menu-"]').first().click();
    await app.locator('.add-menu:not(.hidden-menu) [data-testid^="bill-negotiate-"]').first().click();
    const notes = app.locator("textarea.tc-notes").first();
    await expect(notes).toBeVisible({ timeout: 20_000 });
    await expect(notes).toHaveValue(/competitor/i);
    await expect(notes).toHaveValue(/retention/i);
  });

  test("an upcoming bill still shows whether it fits its budget", async ({ app }) => {
    // Was gapfeatures.spec "bill → budget-fit chip" on /bills. The chip itself
    // survived the redesign intact.
    await nav(app, "/bills");
    const chip = app.locator('[data-testid^="bill-fit-"]').first();
    await expect(chip).toBeVisible({ timeout: 20_000 });
    await expect(chip).toContainText(/fits|over/i);
    await expect(chip).toContainText(/\$[\d,]+\.\d{2}/);
  });

  test("the budget-fit chip drills to the budget it names", async ({ app }) => {
    // RH1, fixed. The redesigned agenda row briefly rendered the fit chip as a
    // plain <span>, losing both the deep-link into the budget it names — with the
    // receiving card flashing — and the chip's accessible name. The chip is a
    // control again, so this guards that it stays one.
    await nav(app, "/bills");
    const chip = app.locator('[data-testid^="bill-fit-"]').first();
    await expect(chip).toBeVisible({ timeout: 20_000 });
    await chip.click();
    await expect(app).toHaveURL(/\/budgets/, { timeout: 10_000 });
    // The receiving budget flashes. Matched by its testid rather than by `.budget`:
    // the sample household has ten budgets, so /budgets auto-seeds its COMPACT list
    // (`.budget-crow`) and the full `.budget` card never renders — a selector that
    // silently depends on which density the page happened to pick is testing the
    // wrong thing. Both densities carry budget-card-<id>, which is also exactly
    // what the deep-link focus targets.
    await expect(app.locator('[data-testid^="budget-card-"].deeplink-flash').first()).toBeVisible({ timeout: 10_000 });
  });

  test("linking a transaction to a subscription persists on the transaction", async ({ app }) => {
    // Was interactions.spec "subscription: mark via the flip modal → subscriptions
    // row shows it and drills to it". The old assertion read `.sub-row .sub-drill`
    // and `sub-pay-*` — the retired subscriptions-panel vocabulary — and the
    // unified surface has no "last paid" line to carry that evidence, so only the
    // linkage half of the intent is still assertable here (the missing half is
    // recorded as a gap). The link itself must still round-trip.
    await nav(app, "/transactions");
    const row = app.locator('[data-testid^="txn-row-"]').nth(6);
    await row.scrollIntoViewIfNeeded();
    await row.locator('[data-testid^="txn-kebab-"]').click();
    await row.locator('[data-testid="txn-marksub-open"]').click();
    await expect(app.getByTestId("txnlink-summary")).toBeVisible();
    await app.waitForTimeout(600); // the FlipPanel's ~550ms 3D flip

    const select = app.getByTestId("txnlink-sub-select");
    await expect(select).toBeVisible();
    const [subValue] = await select.selectOption({ index: 1 });
    expect(subValue).toBeTruthy();
    await app.getByTestId("txnlink-save").click();

    // Reopen: the modal reads back the stored link, so the save was durable.
    await expect(app.getByTestId("txnlink-summary")).toHaveCount(0, { timeout: 20_000 });
    await row.locator('[data-testid^="txn-kebab-"]').click();
    await row.locator('[data-testid="txn-marksub-open"]').click();
    await expect(app.getByTestId("txnlink-summary")).toBeVisible();
    await app.waitForTimeout(600);
    await expect(app.getByTestId("txnlink-sub-select")).toHaveValue(subValue);
  });

  test("a matching transaction settles a scheduled charge", async ({ page, errors }) => {
    void errors;
    // Was the standalone e2e/recurring_match_verify.mjs, which asserted /OVERDUE/
    // and `.rec-tag-paid` — the retired `rec-` vocabulary. Same intent on the new
    // surface: an overdue occurrence stops being overdue once a real payment
    // settles it, via the shared rhySettled predicate.
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    const strip = page.getByTestId("rhy-overdue");
    await expect(strip).toBeVisible();
    await expect(strip).toContainText(/overdue/i);
    const row = strip.locator('[data-testid^="rhy-overdue-row-"]').filter({ hasText: /streaming/i }).first();
    await expect(row).toBeVisible();
    const id = (await row.getAttribute("data-testid")).replace("rhy-overdue-row-", "");
    await page.getByTestId(`rhy-mark-paid-${id}`).click();
    // Settled: gone from the strip, and the calendar agrees it is not a miss.
    await expect(page.getByTestId(`rhy-overdue-row-${id}`)).toHaveCount(0, { timeout: 20_000 });
    await page.getByTestId("rhy-view-calendar").click();
    await expect(page.locator("#main .rhy-cal")).toBeVisible();
    // The other genuinely-missed items stay missed; the settled one is no longer
    // among them. Read the whole set — a not-toContainText on the multi-element
    // locator asks each of them the question and passes on the first that agrees.
    const stillMissed = await page.evaluate(() =>
      [...document.querySelectorAll("#main .rhy-cal-item.is-missed .rhy-cal-name")].map((e) =>
        e.innerText.trim(),
      ),
    );
    expect(stillMissed.length).toBeGreaterThan(0);
    expect(stillMissed.some((n) => /streaming/i.test(n))).toBe(false);
  });
});

test.describe("flip-modal containment (app-wide Height bound)", () => {
  // The Height prop on a flip panel is a MAX bound: tall panels clamp and scroll
  // rather than running off-screen. Sanity-checked on tall modals away from this
  // surface, since the fix was app-wide.
  const TALL = [
    // A tall panel (Detection preferences ran to 1082px inside a 680px wrap) and a
    // sparse one, so both halves of the contract are covered: the bound binds, and
    // a panel that fits still hugs its content.
    ["/recurring", "subs-detect-prefs-toggle", "subs-detect-prefs"],
    ["/recurring", "bills-smart-open", null],
    ["/transactions", "txn-add-open", null],
  ];
  for (const [route, opener, bodyTestId] of TALL) {
    test(`${route} → ${opener} clamps inside the viewport with a reachable footer`, async ({ app }) => {
      await nav(app, route);
      const trigger = app.getByTestId(opener);
      test.skip((await trigger.count()) === 0, `${opener} is not on ${route}`);
      await trigger.first().click();
      const wrap = app.locator(".flip-wrap").first();
      await expect(wrap).toBeVisible({ timeout: 20_000 });
      await app.waitForTimeout(700); // past the ~550ms 3D flip
      if (bodyTestId) await expect(app.getByTestId(bodyTestId)).toBeVisible();

      const vp = app.viewportSize();
      const box = await wrap.boundingBox();
      // The Height prop is a MAX bound: the panel clamps to the viewport rather
      // than running off the bottom of the screen.
      expect(box.y, "panel top is on screen").toBeGreaterThanOrEqual(-1);
      expect(box.y + box.height, "panel bottom is on screen").toBeLessThanOrEqual(vp.height + 1);

      // …and its LAST control is reachable — either already on screen, or after
      // scrolling the panel's OWN scroller (never the page's).
      const reachable = await wrap.evaluate((el) => {
        const btns = [...el.querySelectorAll("button")];
        if (!btns.length) return { ok: true };
        const last = btns[btns.length - 1];
        const onScreen = (n) => {
          const r = n.getBoundingClientRect();
          return r.bottom <= window.innerHeight + 1 && r.top >= -1;
        };
        if (onScreen(last)) return { ok: true, scrolled: false };
        const scroller = [...el.querySelectorAll("*")].find(
          (n) => n.scrollHeight > n.clientHeight + 1 && /auto|scroll/.test(getComputedStyle(n).overflowY),
        );
        if (!scroller) return { ok: false, why: "off screen with no scroller to reach it" };
        const beforeBody = document.documentElement.scrollTop;
        last.scrollIntoView({ block: "end" });
        return {
          ok: onScreen(last),
          scrolled: true,
          bodyMoved: document.documentElement.scrollTop !== beforeBody,
          why: onScreen(last) ? "" : "still off screen after scrolling the panel",
        };
      });
      expect(reachable.ok, `last control reachable — ${reachable.why || ""}`).toBe(true);
      if (reachable.scrolled) {
        // Scrolling inside the panel must not scroll the page behind it.
        expect(reachable.bodyMoved, "the page behind the modal did not scroll").toBe(false);
      }
    });
  }
});

test.describe("surface screenshots", () => {
  test("capture the key states", async ({ app }) => {
    await nav(app, "/recurring");
    await app.screenshot({ path: shot("rhythm_01_surface.png"), fullPage: true });
    await app.getByTestId("rhy-view-calendar").click();
    await expect(app.locator("#main .rhy-cal")).toBeVisible();
    await app.screenshot({ path: shot("rhythm_02_calendar.png"), fullPage: true });
    await app.getByTestId("rhy-view-compact").click();
    await app.getByTestId("rhy-weak-signals").click();
    await expect(app.getByTestId("subs-detect-prefs")).toBeVisible({ timeout: 20_000 });
    await app.screenshot({ path: shot("rhythm_03_detection_prefs.png"), fullPage: true });
    await app.getByTestId("subs-prefs-done").click();
    await nav(app, "/subscriptions");
    await app.screenshot({ path: shot("rhythm_04_subscriptions_lens.png"), fullPage: true });
  });

  test("capture the overdue state", async ({ page, errors }) => {
    void errors;
    await bootAt(page, OVERDUE_NOW);
    await nav(page, "/recurring");
    await expect(page.getByTestId("rhy-overdue")).toBeVisible();
    await page.screenshot({ path: shot("rhythm_05_overdue.png"), fullPage: true });
    await page.getByTestId("rhy-view-calendar").click();
    await expect(page.locator("#main .rhy-cal")).toBeVisible();
    await page.screenshot({ path: shot("rhythm_06_calendar_missed.png"), fullPage: true });
  });
});
