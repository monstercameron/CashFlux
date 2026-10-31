// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import uic "github.com/monstercameron/GoWebComponents/v7/ui"

// IfLazy renders build's node when cond holds, and nothing otherwise. Unlike If, the node is
// only built when cond holds, so a branch that needs statements or a guard before it can
// build its node can stay in one place. The branch must not call hooks: a hook inside a
// conditional build would change the hook order from render to render.
func IfLazy(cond bool, build func() uic.Node) uic.Node {
	if cond {
		return build()
	}
	return uic.Fragment()
}
