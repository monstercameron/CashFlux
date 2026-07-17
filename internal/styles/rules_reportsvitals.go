// SPDX-License-Identifier: MIT

package styles

// registerReportsVitals emits the Annual Review's "00 · Where you stand"
// position snapshot: three ruled ledger columns of vital rows (small-caps
// label, toned display figure, one-line muted reading). The signature device
// is the TARGET-TICK METER — a hairline track whose fill is the metric's toned
// value and whose single faint tick marks the published target (20% savings,
// 36% payment share, 30% utilization, 6-month coverage) — so distance-to-target
// reads spatially before the words do. Theme tokens throughout; prints as three
// fixed columns.
func registerReportsVitals() {
	rawBlock(`.rpta-vitals{display:grid;grid-template-columns:repeat(auto-fit,minmax(250px,1fr));gap:2rem 3rem}
.rpta-vit-col{display:flex;flex-direction:column;gap:1.15rem;min-width:0}
.rpta-vit-head{font-size:.7rem;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--text-faint);padding-bottom:.45rem;border-bottom:1px solid var(--border)}
.rpta-vital{display:flex;flex-direction:column;gap:.22rem}
.rpta-vital-k{font-size:.68rem;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--text-faint)}
.rpta-vital-v{font-size:1.28rem;line-height:1.15;font-weight:600;font-variant-numeric:tabular-nums;color:var(--text)}
.rpta-vital-v.rpta-tone-up{color:var(--up,#4ea777)}
.rpta-vital-v.rpta-tone-warn{color:var(--warn,#d8a24a)}
.rpta-vital-v.rpta-tone-down{color:var(--down,#d8716f)}
.rpta-vital-r{font-size:.78rem;line-height:1.45;color:var(--text-dim);max-width:26rem}
.rpta-vital-meter{position:relative;height:4px;border-radius:999px;background:color-mix(in srgb, var(--border) 60%, transparent);margin:.3rem 0 .1rem;max-width:15rem}
.rpta-vital-fill{position:absolute;top:0;bottom:0;left:0;border-radius:999px}
.rpta-vital-tick{position:absolute;top:-3px;bottom:-3px;width:2px;border-radius:1px;background:var(--text-faint)}
.rpta-vital-empty{margin:0;font-size:.85rem;line-height:1.5;color:var(--text-dim);font-style:italic;max-width:24rem}
.rpta-vitals-basis{margin:1.4rem 0 0;font-size:.75rem}
@media print{.rpta-vitals{grid-template-columns:repeat(3,1fr)}}`)
}
