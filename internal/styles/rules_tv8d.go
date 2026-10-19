// SPDX-License-Identifier: MIT

package styles

func registerTV8D() {
	rule(".split-row",
		display("grid"),
		gap("0.5rem"),
		alignItems("center"),
	)
	rule(".split-editor-modal .modal-scroll > *",
		prop("flex-shrink", "0"),
	)
}
