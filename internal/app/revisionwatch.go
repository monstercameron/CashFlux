// SPDX-License-Identifier: MIT

package app

// revisionWatcher decides, one tick at a time, when the autosaved dataset should
// be written. It is the scheduling half of C584's fix, kept out of the wasm-only
// persistence file so the policy can be tested on native Go.
//
// The policy is leading-edge plus trailing-edge, the same shape the settings
// persist already uses:
//
//   - LEADING: the first tick that sees a new revision writes immediately. A
//     single edit — the overwhelmingly common case — is therefore durable within
//     one tick, not two. The earlier quiet-only policy waited for a tick of
//     silence AFTER the tick that noticed the change, which doubled the window in
//     which a refresh could still lose the edit. That residual window is what the
//     C598 journey caught: save an inline limit, refresh at once, get the old
//     number back.
//   - TRAILING: once the revision stops moving, one more write catches whatever
//     landed after the leading-edge serialize. It is nearly free — the caller
//     compares the serialized bytes and skips an unchanged write.
//
// A burst therefore costs at most one write per tick rather than one per edit,
// while a lone edit no longer pays for the burst case that did not happen.
type revisionWatcher struct {
	seen  int
	burst bool
}

// tick folds in the revision observed now and reports whether to write.
func (w *revisionWatcher) tick(rev int) bool {
	switch {
	case rev != w.seen:
		w.seen, w.burst = rev, true
		return true
	case w.burst:
		w.burst = false
		return true
	}
	return false
}
