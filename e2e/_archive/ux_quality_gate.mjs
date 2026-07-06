// ux_quality_gate.mjs — R44/R72 desktop UX quality gate (unified scorecard).
//
// Runs all four UX quality-audit dimensions against a running app and prints a
// single pass/fail scorecard. Each child audit follows the style spec's §11.1
// measurement protocol and exits with its failure count; this runner aggregates
// them and exits non-zero if any dimension fails, so it can gate CI in one call.
//
//   Dimension   Spec      Script
//   contrast    §12       ux_contrast_audit.mjs
//   density     §11       ux_density_audit.mjs
//   overflow    §5.5.11   ux_overflow_audit.mjs
//
// CashFlux is desktop-first (no mobile), so this headline gate covers the three
// desktop dimensions. The coarse-pointer touch-target audit (ux_touch_audit.mjs,
// §5.5.9 — for touchscreen laptops/hybrids) is a separate optional check, not run
// here.
//
// Usage:  node e2e/ux_quality_gate.mjs [baseURL]
//   (serve the app first, e.g. `go run e2e/serve.go`).

import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const baseArg = process.argv[2] || 'http://127.0.0.1:8099/';

const DIMS = [
  { key: 'contrast', spec: '§12',     script: 'ux_contrast_audit.mjs' },
  { key: 'density',  spec: '§11',     script: 'ux_density_audit.mjs' },
  { key: 'overflow', spec: '§5.5.11', script: 'ux_overflow_audit.mjs' },
  { key: 'parity',   spec: '§12.1',   script: 'ux_theme_parity_audit.mjs' },
];

function run(script) {
  return new Promise((resolve) => {
    const child = spawn(process.execPath, [join(here, script), baseArg], { stdio: ['ignore', 'pipe', 'pipe'] });
    let tail = '';
    child.stdout.on('data', (d) => { tail = (tail + d.toString()).split('\n').slice(-4).join('\n'); });
    child.stderr.on('data', (d) => { tail = (tail + d.toString()).split('\n').slice(-4).join('\n'); });
    child.on('close', (code) => resolve({ code: code ?? -1, tail: tail.trim() }));
  });
}

console.log(`CashFlux desktop UX quality gate — ${baseArg}\n`);
const results = [];
for (const d of DIMS) {
  process.stdout.write(`  running ${d.key} (${d.spec}) … `);
  const r = await run(d.script);
  const pass = r.code === 0;
  console.log(pass ? 'PASS' : `FAIL (${r.code})`);
  results.push({ ...d, ...r, pass });
}

console.log('\n────────────────────────────────────────');
console.log(' dimension   spec      result');
console.log('────────────────────────────────────────');
let failed = 0;
for (const r of results) {
  if (!r.pass) failed++;
  const summary = (r.tail.split('\n').pop() || '').slice(0, 38);
  console.log(` ${r.key.padEnd(10)} ${r.spec.padEnd(9)} ${r.pass ? 'PASS' : 'FAIL ' + r.code}  ${summary}`);
}
console.log('────────────────────────────────────────');
console.log(failed === 0
  ? '\n✅ ALL DIMENSIONS PASS — desktop UX quality gate green.'
  : `\n❌ ${failed}/${results.length} dimension(s) failing.`);

process.exit(Math.min(failed, 250));
