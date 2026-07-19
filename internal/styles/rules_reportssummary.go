// SPDX-License-Identifier: MIT

package styles

// registerReportsSummary emits the executive Summary view of the Annual Review
// (/reports) plus its Summary/Full mode toggle. The Summary is the default: a
// one-to-two-viewport digest — top strengths / risks / actions in three toned
// columns, then a compact core-trends strip — that reuses the figures the full
// report already computes. Full mode reveals the long-form document as the
// supporting evidence. Theme tokens throughout so both themes read cleanly. The
// zone tones (.rpta-tone-*, .rpta-z-*) are defined once in registerReportsAnnual.
func registerReportsSummary() {
	// ── The mode toggle: a compact segmented control at the top of the report.
	rawBlock(`.rpta-modes{display:inline-flex;gap:0.25rem;align-self:flex-start;padding:0.25rem;border:1px solid var(--border);border-radius:0.6rem;background:var(--bg-elev)}
.rpta-mode{appearance:none;border:0;background:transparent;color:var(--text-dim);font:inherit;font-weight:600;font-size:0.85rem;padding:0.4rem 0.9rem;border-radius:0.45rem;cursor:pointer;transition:background .15s,color .15s}
.rpta-mode:hover{color:var(--text)}
.rpta-mode.is-on{background:var(--accent);color:var(--accent-contrast,#fff)}
.rpta-mode:focus-visible{outline:2px solid var(--accent);outline-offset:2px}`)

	// ── The summary body: three toned columns + a core-trends strip.
	rawBlock(`.rpta-summary{display:flex;flex-direction:column;gap:2rem;width:100%}
.rpta-sum-cols{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:1.25rem;align-items:start}
.rpta-sum-col{display:flex;flex-direction:column;gap:0.9rem;padding:1.1rem 1.15rem;border:1px solid var(--border);border-left:3px solid var(--rpta-zone,var(--border));border-radius:0.7rem;background:var(--bg-elev)}
.rpta-sum-col-title{margin:0;font-size:0.72rem;font-weight:700;letter-spacing:0.1em;text-transform:uppercase}
.rpta-sum-list{display:flex;flex-direction:column;gap:0.85rem}
.rpta-sum-list .rpta-win{display:flex;gap:0.4rem;align-items:flex-start;font-size:0.92rem;line-height:1.35}
.rpta-sum-list .rpta-plan-item{gap:0.6rem}
.rpta-sum-list .rpta-fact{margin:0}`)

	// ── Core trends: five compact stat cells, each with an optional sparkline.
	rawBlock(`.rpta-sum-trends-wrap{display:flex;flex-direction:column;gap:0.75rem;padding-top:1.25rem;border-top:1px solid var(--border)}
.rpta-sum-trends-title{margin:0;font-size:0.72rem;font-weight:700;letter-spacing:0.1em;text-transform:uppercase;color:var(--text-dim)}
.rpta-sum-trends{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:1rem}
.rpta-sum-trend{display:flex;flex-direction:column;gap:0.2rem;padding:0.9rem 1rem;border:1px solid var(--border);border-radius:0.6rem;background:var(--bg-elev);min-width:0}
.rpta-sum-trend-k{font-size:0.7rem;font-weight:600;letter-spacing:0.08em;text-transform:uppercase;color:var(--text-faint)}
.rpta-sum-trend-val{font-size:1.35rem;line-height:1.15;font-variant-numeric:tabular-nums;overflow-wrap:anywhere}
.rpta-sum-trend-sub{font-size:0.75rem}
.rpta-sum-trend-spark{margin-top:0.35rem;height:1.5rem;color:var(--accent)}
.rpta-sum-trend-spark svg{width:100%;height:100%}
.rpta-sum-fulllink{align-self:flex-start}
.rpta-appendix-fold>summary{cursor:pointer;font-weight:600;color:var(--text-dim);padding:0.25rem 0}
.rpta-appendix-fold>summary:hover{color:var(--text)}
.rpta-appendix-body{display:flex;flex-direction:column;gap:2rem;padding-top:1.25rem}`)

	// Stack the columns and trend cells on narrow content widths.
	rawBlockMedia("(max-width:860px)", `.rpta-sum-cols{grid-template-columns:1fr}
.rpta-sum-trends{grid-template-columns:repeat(2,minmax(0,1fr))}`)
}
