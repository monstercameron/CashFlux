// SPDX-License-Identifier: MIT

package rpcworker

import "testing"

func TestWorkerURLFromBase(t *testing.T) {
	for _, tc := range []struct {
		base, want string
		bad        bool
	}{
		{base: "https://budget.example/app/", want: "https://budget.example/app/services-worker.js"},
		{base: "http://127.0.0.1:8099/settings/cloud", want: "http://127.0.0.1:8099/settings/services-worker.js"},
		{base: "", bad: true},
		{base: "about:blank", bad: true},
		{base: ":invalid", bad: true},
	} {
		got, err := workerURLFromBase(tc.base)
		if (err != nil) != tc.bad || got != tc.want {
			t.Errorf("workerURLFromBase(%q) = %q, %v; want %q, bad=%v", tc.base, got, err, tc.want, tc.bad)
		}
	}
}
