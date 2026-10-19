// SPDX-License-Identifier: MIT

package txntable

// MenuState carries transaction facts and the actions its host can perform.
type MenuState struct {
	Automatic, Transfer, Split, Excluded, Income, Refund, Refunded, BillMatched                           bool
	GroupSize                                                                                             int
	Confirm, Suggest, OpenSplit, ToggleExclude, Delete, OpenLink, PairRefund, Unpair, Ungroup, UnlinkBill bool
}

// Menu contains the enabled entries and their current-state labels.
type Menu struct {
	Confirm, Suggest, Split, Exclude, Delete, Link, Pair, Unpair, Ungroup, UnlinkBill bool
	SplitChecked, Include                                                             bool
}

// DecideMenu selects available row actions without invoking callbacks.
func DecideMenu(s MenuState) Menu {
	return Menu{Confirm: s.Automatic && s.Confirm, Suggest: s.Suggest && !s.Transfer, Split: s.OpenSplit && !s.Transfer,
		Exclude: s.ToggleExclude, Delete: s.Delete, Link: s.OpenLink,
		Pair: s.PairRefund && s.Income && !s.Refund && !s.Transfer, Unpair: s.Unpair && (s.Refund || s.Refunded),
		Ungroup: s.Ungroup && s.GroupSize > 1, UnlinkBill: s.UnlinkBill && s.BillMatched, SplitChecked: s.Split, Include: s.Excluded}
}
