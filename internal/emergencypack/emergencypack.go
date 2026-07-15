// SPDX-License-Identifier: MIT

// Package emergencypack builds the "in case of emergency" document (AC16): a single,
// plain-language pack a spouse or executor can read if something happens to you —
// which accounts exist, at which institutions, how to reach those institutions, what
// documents are filed, and any beneficiary / transfer-on-death notes you left. It is
// deliberately a pure renderer with no platform dependencies, so it unit-tests on
// native Go and — critically — makes ZERO network calls: the pack is generated on
// your own machine and never leaves it unless you choose to print or save it.
//
// PRIVACY CONTRACT (do not weaken):
//   - The pack NEVER contains passwords, PINs, or login credentials. Those live only
//     in the encrypted credential vault; this document merely NOTES that a vault
//     exists and how the executor can find it. Callers must not pass secrets in.
//   - Everything here is assembled from data the user already stored locally; the
//     renderer adds no external lookups.
//
// The tone is intentionally warm and calm — this is read at a hard moment.
package emergencypack

import (
	"html"
	"sort"
	"strings"
	"time"
)

// Contact is one person the reader may need to reach — an attorney, financial
// advisor, the named executor, a trusted relative.
type Contact struct {
	Name  string
	Role  string
	Phone string
	Email string
	Note  string
}

// DocEntry is one filed document listed in the pack (never the document's contents —
// just that it exists and where it is filed in the app).
type DocEntry struct {
	Label      string
	AttachedOn time.Time
	ExpiresOn  time.Time // zero when the document does not expire
}

// AccountEntry is one account as it appears in the pack. Balance is a PRE-FORMATTED
// money string (the caller formats through the app's money formatter) — the pure
// package never guesses currency formatting. BeneficiaryNote and Notes are the
// user's own plain-text notes; Documents lists filed paperwork.
type AccountEntry struct {
	Name            string
	Institution     string
	Type            string
	Balance         string
	BeneficiaryNote string
	Notes           string
	Documents       []DocEntry
}

// InstitutionEntry is one institution and how to reach its support line. SupportURL
// is a public web address only — never a stored login.
type InstitutionEntry struct {
	Name         string
	SupportPhone string
	SupportURL   string
	Note         string
	Accounts     []string // account names held here, for the roll-up
}

// Pack is the fully-assembled input to the renderers. The caller gathers it from the
// local dataset; the renderers only format it.
type Pack struct {
	GeneratedAt time.Time
	OwnerName   string
	// VaultHasEntries is true when the credential vault holds at least one login, so
	// the pack can point the reader to it WITHOUT ever revealing its contents.
	VaultHasEntries bool
	Accounts        []AccountEntry
	Institutions    []InstitutionEntry
	Contacts        []Contact
	// ClosingNote is an optional personal message the user adds to the reader.
	ClosingNote string
}

const (
	title       = "In case of emergency"
	intro       = "This document was prepared by CashFlux on this computer so that someone you trust can find their way around your finances if you are not able to help. Take your time. Nothing here needs to be done today."
	vaultLine   = "Your saved logins are kept in CashFlux's encrypted vault, which you can open in the app. For your safety, no sign-in details are printed in this document."
	noVaultLine = "No saved logins are stored in CashFlux. You may need to contact each institution directly to gain access."
	safetyLine  = "This document may list account names and balances. Please keep it somewhere private and secure."
)

// vaultParagraph returns the credential-vault sentence appropriate to whether the
// vault holds anything, so the reader is pointed to it only when it is useful.
func (p Pack) vaultParagraph() string {
	if p.VaultHasEntries {
		return vaultLine
	}
	return noVaultLine
}

// fmtDate renders a date as "2 Jan 2006", or "" for the zero time.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2 Jan 2006")
}

// RenderText produces the pack as plain UTF-8 text — the most portable form, safe to
// print or save anywhere. It never contains credentials.
func RenderText(p Pack) string {
	var b strings.Builder
	line := func(s string) { b.WriteString(s); b.WriteByte('\n') }

	line(strings.ToUpper(title))
	if p.OwnerName != "" {
		line("Prepared for the family of " + p.OwnerName)
	}
	if !p.GeneratedAt.IsZero() {
		line("Prepared on " + fmtDate(p.GeneratedAt))
	}
	line("")
	line(intro)
	line("")
	line(p.vaultParagraph())
	line(safetyLine)

	line("")
	line("— ACCOUNTS —")
	if len(p.Accounts) == 0 {
		line("(No accounts were recorded.)")
	}
	for _, a := range p.Accounts {
		line("")
		head := "• " + a.Name
		if a.Institution != "" {
			head += "  (" + a.Institution + ")"
		}
		line(head)
		if a.Type != "" {
			line("   Type: " + a.Type)
		}
		if a.Balance != "" {
			line("   Recent balance: " + a.Balance)
		}
		if strings.TrimSpace(a.BeneficiaryNote) != "" {
			line("   Beneficiary / transfer-on-death: " + a.BeneficiaryNote)
		}
		if strings.TrimSpace(a.Notes) != "" {
			line("   Notes: " + a.Notes)
		}
		for _, d := range a.Documents {
			docLine := "   Document on file: " + d.Label
			if on := fmtDate(d.AttachedOn); on != "" {
				docLine += " (filed " + on + ")"
			}
			if ex := fmtDate(d.ExpiresOn); ex != "" {
				docLine += " — renews/expires " + ex
			}
			line(docLine)
		}
	}

	line("")
	line("— INSTITUTIONS —")
	if len(p.Institutions) == 0 {
		line("(No institutions were recorded.)")
	}
	for _, in := range p.Institutions {
		line("")
		line("• " + in.Name)
		if in.SupportPhone != "" {
			line("   Support phone: " + in.SupportPhone)
		}
		if in.SupportURL != "" {
			line("   Support website: " + in.SupportURL)
		}
		if len(in.Accounts) > 0 {
			line("   Accounts held here: " + strings.Join(in.Accounts, ", "))
		}
		if strings.TrimSpace(in.Note) != "" {
			line("   Notes: " + in.Note)
		}
	}

	if len(p.Contacts) > 0 {
		line("")
		line("— PEOPLE TO CONTACT —")
		for _, c := range p.Contacts {
			line("")
			head := "• " + c.Name
			if c.Role != "" {
				head += "  (" + c.Role + ")"
			}
			line(head)
			if c.Phone != "" {
				line("   Phone: " + c.Phone)
			}
			if c.Email != "" {
				line("   Email: " + c.Email)
			}
			if strings.TrimSpace(c.Note) != "" {
				line("   Notes: " + c.Note)
			}
		}
	}

	if strings.TrimSpace(p.ClosingNote) != "" {
		line("")
		line("— A NOTE FROM " + strings.ToUpper(orName(p.OwnerName)) + " —")
		line(p.ClosingNote)
	}
	return b.String()
}

func orName(n string) string {
	if strings.TrimSpace(n) == "" {
		return "the account holder"
	}
	return n
}

// RenderHTML produces a self-contained, printable HTML document (all styles inline
// in a <style> block, no external assets or scripts, no network calls). Every piece
// of user text is HTML-escaped. It never contains credentials.
func RenderHTML(p Pack) string {
	e := html.EscapeString
	var b strings.Builder
	w := func(s string) { b.WriteString(s) }

	w("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">")
	w("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	w("<title>" + e(title) + "</title><style>")
	w("body{font:16px/1.6 -apple-system,Segoe UI,Roboto,sans-serif;color:#1a1a1a;background:#fff;max-width:44rem;margin:2rem auto;padding:0 1.25rem}")
	w("h1{font-size:1.7rem;margin:0 0 .25rem}h2{font-size:1.1rem;border-bottom:1px solid #ddd;padding-bottom:.3rem;margin-top:2rem}")
	w(".meta{color:#555;margin:0 0 1.25rem}.intro{background:#f5f7fa;border-radius:.6rem;padding:1rem 1.15rem;margin:1rem 0}")
	w(".safe{color:#555;font-size:.92rem}.entry{margin:1rem 0;padding-left:.25rem}.entry .name{font-weight:600}")
	w(".field{margin:.15rem 0}.field .k{color:#555}.doc{color:#333;font-size:.94rem;margin:.1rem 0}")
	w(".closing{background:#fbf8f2;border-radius:.6rem;padding:1rem 1.15rem;margin-top:1.5rem;white-space:pre-wrap}")
	w("@media print{body{margin:0}}</style></head><body>")

	w("<h1>" + e(title) + "</h1>")
	var meta []string
	if p.OwnerName != "" {
		meta = append(meta, "Prepared for the family of "+e(p.OwnerName))
	}
	if !p.GeneratedAt.IsZero() {
		meta = append(meta, "Prepared on "+e(fmtDate(p.GeneratedAt)))
	}
	if len(meta) > 0 {
		w("<p class=\"meta\">" + strings.Join(meta, " · ") + "</p>")
	}
	w("<div class=\"intro\"><p>" + e(intro) + "</p><p>" + e(p.vaultParagraph()) + "</p></div>")
	w("<p class=\"safe\">" + e(safetyLine) + "</p>")

	w("<h2>Accounts</h2>")
	if len(p.Accounts) == 0 {
		w("<p class=\"safe\">No accounts were recorded.</p>")
	}
	for _, a := range p.Accounts {
		w("<div class=\"entry\"><div class=\"name\">" + e(a.Name))
		if a.Institution != "" {
			w(" <span class=\"k\">— " + e(a.Institution) + "</span>")
		}
		w("</div>")
		field := func(k, v string) {
			if strings.TrimSpace(v) != "" {
				w("<div class=\"field\"><span class=\"k\">" + e(k) + ":</span> " + e(v) + "</div>")
			}
		}
		field("Type", a.Type)
		field("Recent balance", a.Balance)
		field("Beneficiary / transfer-on-death", a.BeneficiaryNote)
		field("Notes", a.Notes)
		for _, d := range a.Documents {
			line := d.Label
			if on := fmtDate(d.AttachedOn); on != "" {
				line += " (filed " + on + ")"
			}
			if ex := fmtDate(d.ExpiresOn); ex != "" {
				line += " — renews/expires " + ex
			}
			w("<div class=\"doc\">Document on file: " + e(line) + "</div>")
		}
		w("</div>")
	}

	w("<h2>Institutions</h2>")
	if len(p.Institutions) == 0 {
		w("<p class=\"safe\">No institutions were recorded.</p>")
	}
	for _, in := range p.Institutions {
		w("<div class=\"entry\"><div class=\"name\">" + e(in.Name) + "</div>")
		if in.SupportPhone != "" {
			w("<div class=\"field\"><span class=\"k\">Support phone:</span> " + e(in.SupportPhone) + "</div>")
		}
		if in.SupportURL != "" {
			w("<div class=\"field\"><span class=\"k\">Support website:</span> " + e(in.SupportURL) + "</div>")
		}
		if len(in.Accounts) > 0 {
			w("<div class=\"field\"><span class=\"k\">Accounts held here:</span> " + e(strings.Join(in.Accounts, ", ")) + "</div>")
		}
		if strings.TrimSpace(in.Note) != "" {
			w("<div class=\"field\"><span class=\"k\">Notes:</span> " + e(in.Note) + "</div>")
		}
		w("</div>")
	}

	if len(p.Contacts) > 0 {
		w("<h2>People to contact</h2>")
		for _, c := range p.Contacts {
			w("<div class=\"entry\"><div class=\"name\">" + e(c.Name))
			if c.Role != "" {
				w(" <span class=\"k\">— " + e(c.Role) + "</span>")
			}
			w("</div>")
			if c.Phone != "" {
				w("<div class=\"field\"><span class=\"k\">Phone:</span> " + e(c.Phone) + "</div>")
			}
			if c.Email != "" {
				w("<div class=\"field\"><span class=\"k\">Email:</span> " + e(c.Email) + "</div>")
			}
			if strings.TrimSpace(c.Note) != "" {
				w("<div class=\"field\"><span class=\"k\">Notes:</span> " + e(c.Note) + "</div>")
			}
			w("</div>")
		}
	}

	if strings.TrimSpace(p.ClosingNote) != "" {
		w("<h2>A note from " + e(orName(p.OwnerName)) + "</h2>")
		w("<div class=\"closing\">" + e(p.ClosingNote) + "</div>")
	}

	w("</body></html>")
	return b.String()
}

// SortAccountsForPack orders account entries by institution then name so the pack
// reads as an organized roll-up ("everything about Chase together").
func SortAccountsForPack(entries []AccountEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Institution != entries[j].Institution {
			return entries[i].Institution < entries[j].Institution
		}
		return entries[i].Name < entries[j].Name
	})
}
