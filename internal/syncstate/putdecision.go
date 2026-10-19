// SPDX-License-Identifier: MIT

package syncstate

import "time"

// putdecision.go decides whether the server accepts an incoming workspace push.
//
// It used to be one line: reject when the client's timestamp is Before the
// stored one. That compares two DIFFERENT CLOCKS. clientUpdatedAt is
// time.Now() in a browser; the stored UpdatedAt is time.Now() on the server,
// stamped when some earlier push landed. A device whose clock runs even slightly
// behind the server's therefore loses EVERY push, forever, filing a conflict
// backup each time — which is indistinguishable from "sync stopped working"
// (Cam, deployed instance, 2026-08-31).
//
// The server already keeps exactly the right thing: Version, a counter it
// increments on every accepted write. A client that has pulled or pushed knows
// the version its edit was based on. Comparing versions is optimistic
// concurrency — clock-free, skew-proof, and it answers the question the
// timestamp was only ever approximating: "has anything landed since the copy I
// edited?"
//
// The timestamp path stays for clients that do not send a base version, because
// a server upgrade must not stop older clients syncing.

// PutInput is everything the accept/reject decision needs.
type PutInput struct {
	// Exists is whether the server already holds this workspace.
	Exists bool
	// Deleted is whether the held workspace is a tombstone. A non-Force push
	// must never resurrect one: the delete was itself a write (it bumped the
	// version), and a queued push from before it landing afterwards would
	// silently undo it.
	Deleted bool
	// Force skips the check entirely — the user has explicitly chosen to
	// overwrite, having been told what they are overwriting.
	Force bool
	// ClientBaseVersion is the version the client's edit was based on. Zero
	// means the client did not send one (an older build), which selects the
	// legacy timestamp path rather than being treated as "version 0".
	ClientBaseVersion int64
	// HasBaseVersion is false only for peers that predate versioned writes.
	HasBaseVersion bool
	// CurrentVersion is the version the server holds.
	CurrentVersion int64
	// ClientUpdatedAt and CurrentUpdatedAt are only consulted on the legacy path.
	ClientUpdatedAt  time.Time
	CurrentUpdatedAt time.Time
}

// PutVerdict is the decision plus why, so the caller can log and the client can
// tell a stale push apart from a rejected one.
type PutVerdict struct {
	Accept bool
	// Reason is a short machine-readable tag: "new", "forced", "deleted",
	// "version-match", "version-stale", "version-ahead", "clock-newer",
	// "clock-older".
	Reason string
}

// DecidePut answers whether to accept an incoming push.
//
// Order matters. A workspace the server does not hold yet is always accepted —
// there is nothing to lose a race against. Force is next, because it is the
// user's own explicit decision (including resurrecting a deleted workspace on
// purpose). A tombstone then refuses every ordinary push. Only then does
// concurrency apply, by version when the client supplied one and by clock
// when it did not.
func DecidePut(in PutInput) PutVerdict {
	if !in.Exists {
		return PutVerdict{Accept: true, Reason: "new"}
	}
	if in.Force {
		return PutVerdict{Accept: true, Reason: "forced"}
	}
	if in.Deleted {
		return PutVerdict{Accept: false, Reason: "deleted"}
	}
	if in.HasBaseVersion || in.ClientBaseVersion > 0 {
		// Optimistic concurrency, exact match only. Equal means nothing has
		// landed since the copy this edit was based on, so the edit is a valid
		// successor.
		//
		// Less is stale: another write landed that this edit never saw, and
		// accepting would clobber it.
		//
		// Greater is rejected too ("version-ahead"). It means the client saw a
		// version this server no longer reports — the server was restored from
		// a backup or rolled back, or the queue belongs to a different server.
		// Accepting would let a stale or foreign queue overwrite the restored
		// data with no one having decided that. Rejecting is not stranding:
		// the rejection carries the server's current state, the client pulls,
		// and its next edit is based on a version that exists here. If a
		// person decides the client's copy should win anyway, that is what
		// Force is for.
		switch {
		case in.ClientBaseVersion == in.CurrentVersion:
			return PutVerdict{Accept: true, Reason: "version-match"}
		case in.ClientBaseVersion < in.CurrentVersion:
			return PutVerdict{Accept: false, Reason: "version-stale"}
		default:
			return PutVerdict{Accept: false, Reason: "version-ahead"}
		}
	}
	// Legacy client: no base version, so fall back to the clocks. Kept exactly as
	// it behaved before — including accepting on an exact tie — because changing
	// it would alter the outcome for builds that cannot send a version and cannot
	// be fixed from here.
	if in.ClientUpdatedAt.Before(in.CurrentUpdatedAt) {
		return PutVerdict{Accept: false, Reason: "clock-older"}
	}
	return PutVerdict{Accept: true, Reason: "clock-newer"}
}
