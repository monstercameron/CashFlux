// ux_token_roles_audit.mjs — R46 semantic theme-adherence audit (token roles).
//
// CASHFLUX_ENTERPRISE_UI_STYLE_SPEC §4.1/§4.3 requires color to carry ONE meaning
// per context: brand accent (--accent), positive/negative MONEY semantics
// (--money-positive / --money-negative, aliasing --up/--down), and warning/critical
// SEVERITY (--warn / --danger) are distinct token ROLES. The frequent failure is a
// money figure painted with the brand accent or the severity-danger token, so green
// "brand" and green "you gained money" become indistinguishable, and a red balance
// reads the same as a red critical alert.
//
// This is a SOURCE audit (no browser): it parses web/index.html's CSS and, for every
// rule whose selector targets a MONEY-DIRECTION element (.pos / .neg / .text-up /
// .text-down / .amount-positive|negative / .*-value.pos|neg / hero money values),
// flags any `color:` declaration that uses the WRONG role token — var(--accent) or
// var(--danger) — or a raw hardcoded hex, instead of the money tokens. Keeping money
// semantics on the money tokens is a near-zero-visual change (the tokens resolve to
// the tuned up/down colors) but makes the role correct and centrally themeable.
//
// The check is bidirectional: money figures must not borrow the brand/severity
// tokens, AND severity status elements (.is-warning/.is-critical/.card-alert/…) must
// not borrow the money tokens — a critical alert is not "negative money". (Brand
// accent legitimately also serves the interactive/selected-nav role family, and
// passive chrome uses the bg/border tokens; those are already cleanly separated, so
// the money<->severity boundary is the one with historic violations and the one this
// audit guards.)
//
// Usage:  node e2e/ux_token_roles_audit.mjs [path/to/index.html]
// Exit code = number of role-token violations (money-on-wrong-token + severity-on-money).

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const file = process.argv[2] || join(here, '..', 'web', 'index.html');
const css = readFileSync(file, 'utf8');

// Selectors that denote a MONEY-direction value (positive/negative money figure).
// Deliberately narrow: .pos/.neg are the money up/down modifiers; the *-value/
// *-net/*-amount/text-up/down families are money figures. We do NOT flag generic
// status classes (.is-warning/.is-critical) — those legitimately use severity tokens.
const MONEY_SEL = /(\.(text-up|text-down|amount-positive|amount-negative|amount-income|amount-expense)\b)|((hero-net|hero-net-delta|hero-flanker-value|hero-stat-value|hero-stat-sub|stat-value|kpi-value|t-figure|money|amount)[\w-]*\.(pos|neg)\b)|(\.(pos|neg)\b\s*$)/;
// The right-role tokens for money (either positive or negative side). Includes the
// raw --up/--down aliases, since `--money-positive: var(--up)` / `--money-negative:
// var(--down)` — so a severity element borrowing `var(--down)` is still borrowing
// the money token and must be caught.
const MONEY_TOKEN = /var\(--(money-(positive|negative)|up|down)\b/;
// Wrong-role tokens when used on a money-direction selector.
const WRONG_TOKEN = /var\(--(accent|danger)\b/;
const RAW_HEX = /:\s*#[0-9a-fA-F]{3,8}\b/;

// Crude CSS rule splitter good enough for the project's hand-authored <style>: split
// on '}' and pair the selector list (before '{') with its declaration block.
const rules = [];
for (const chunk of css.split('}')) {
  const i = chunk.lastIndexOf('{');
  if (i === -1) continue;
  const sel = chunk.slice(0, i).split('\n').pop().trim();
  const body = chunk.slice(i + 1).trim();
  if (sel && body) rules.push({ sel, body });
}

// Severity (warning/critical) status selectors — the OTHER side of the boundary.
// These must use the severity tokens (--warn / --danger), never the money tokens:
// a critical alert is not "negative money". We check the inverse contamination so
// the audit enforces the money<->severity role split in BOTH directions (§4.1).
const SEVERITY_SEL = /\.(is-warning|is-critical|is-danger|alert|toast-err|card-alert|budget-over)\b/;

function colorDecl(body) {
  const m = body.match(/(?:^|;|\{)\s*color\s*:\s*([^;]+)/);
  return m ? m[1].trim() : null;
}

const violations = [];
for (const { sel, body } of rules) {
  // Money figures must use money tokens, never brand-accent / severity-danger / raw hex.
  if (MONEY_SEL.test(sel)) {
    const decl = colorDecl(body);
    if (decl) {
      // Only the figure's own ink — background tints (color-mix washes) are theme-correct.
      if (MONEY_TOKEN.test(decl)) { /* correct */ }
      else if (WRONG_TOKEN.test(decl)) violations.push({ sel, decl, why: 'wrong role token (accent/danger) on money figure' });
      else if (RAW_HEX.test('color:' + decl)) violations.push({ sel, decl, why: 'hardcoded hex on money figure' });
    }
  }
  // Severity status elements must not borrow money tokens for their text ink.
  if (SEVERITY_SEL.test(sel)) {
    const decl = colorDecl(body);
    if (decl && MONEY_TOKEN.test(decl)) {
      violations.push({ sel, decl, why: 'money token on severity element (severity != money)' });
    }
  }
}

console.log(`R46 token-role audit — ${file}\n`);
if (violations.length === 0) {
  console.log('✅ 0 money-direction color declarations on the wrong role token.');
} else {
  console.log(`❌ ${violations.length} money-direction color declaration(s) on a non-money token:\n`);
  for (const v of violations) console.log(`  ${v.sel}\n     color: ${v.decl}   <- ${v.why}`);
}
process.exit(Math.min(violations.length, 250));
