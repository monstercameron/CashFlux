# Tax reference source verification

Verified **2026-10-21** for the existing 2025 and 2026 entries in
`internal/taxref/taxref.go`. All monetary amounts below are whole US dollars.
This records reference facts; it does not determine anyone's eligibility,
allowable deduction, or contribution.

## Verified values

| Existing field | 2025 | 2026 | Primary source |
| --- | ---: | ---: | --- |
| 401(k) employee contribution | 23,500 | 24,500 | R25 / R26 |
| General age-50 catch-up | 7,500 | 8,000 | R25 / R26 |
| IRA contribution | 7,000 | 7,500 | R25 / R26 |
| IRA age-50 catch-up | 1,000 | 1,100 | R25 / R26 |
| Retirement catch-up age | 50 | 50 | R25 / R26 |
| HSA self-only contribution | 4,300 | 4,400 | H |
| HSA family contribution | 8,550 | 8,750 | H |
| Additional HSA contribution | 1,000 | 1,000 | H |
| HSA additional-contribution age | 55 | 55 | H |
| Standard deduction, single | **15,750** | **16,100** | D, sections 3.01 / 4.14 |
| Standard deduction, married jointly | **31,500** | **32,200** | D, sections 3.01 / 4.14 |
| Social Security taxable maximum | 176,100 | 184,500 | S |
| Full retirement age, born 1960 or later | 67 | 67 | A |
| Earliest retirement claim age | 62 | 62 | A |
| Age when delayed credits stop | 70 | 70 | C |

The four standard-deduction fields were incorrect. The 2025 entry retained
superseded amounts; the 2026 entry repeated the amended 2025 amounts. The other
existing numerical fields match the cited publications.

The summary now identifies the 401(k) catch-up as the **general** age-50 limit:
the IRS sources also describe a higher amount for ages 60-63. The summary also
states the birth-year qualification attached to the full retirement age.
These qualifications describe the scope of the existing fields without adding
an eligibility calculator or additional tax categories.

## Primary sources

- **R25:** [IRS IR-2024-285, November 1, 2024: retirement contribution limits for 2025](https://www.irs.gov/newsroom/401k-limit-increases-to-23500-for-2025-ira-limit-remains-7000).
- **R26:** [IRS IR-2025-111, November 13, 2025: retirement contribution limits for 2026](https://www.irs.gov/newsroom/401k-limit-increases-to-24500-for-2026-ira-limit-increases-to-7500).
- **H:** [IRS Publication 969 (2025), Limit on Contributions and Additional contribution](https://www.irs.gov/publications/p969). This publication states both the 2025 and 2026 HSA limits.
- **D:** [IRS Revenue Procedure 2025-32, Internal Revenue Bulletin 2025-45, November 3, 2025](https://www.irs.gov/irb/2025-45_IRB). Section 3.01 replaces the earlier 2025 standard deductions; section 4.14 gives the 2026 amounts.
- **S:** [Social Security Administration, Contribution and Benefit Base](https://www.ssa.gov/oact/cola/cbb.html), historical rows for 2025 and 2026.
- **A:** [Social Security Administration, retirement benefits for people born in 1960 or later](https://www.ssa.gov/benefits/retirement/planner/1960.html).
- **C:** [Social Security Administration, Delayed Retirement Credits](https://www.ssa.gov/benefits/retirement/planner/delayret.html).

Social Security values use SSA's primary publications. Tax and contribution
values use IRS primary publications. Each entry carries its verification date
and source URLs through the tool's existing `Limits.Summary` response. No lookup
occurs at runtime, and unsupported tax years continue to be refused.
