// SPDX-License-Identifier: MIT

package appstate

import "testing"

func TestPayoffTracking(t *testing.T) {
	a := newApp(t, false)

	if _, _, tracking := a.PayoffProgress(50000); tracking {
		t.Fatal("no baseline yet — tracking should be false")
	}

	if err := a.StartPayoffTracking(100000, "USD"); err != nil {
		t.Fatalf("StartPayoffTracking: %v", err)
	}
	prog, since, tracking := a.PayoffProgress(60000)
	if !tracking {
		t.Fatal("tracking should be active after StartPayoffTracking")
	}
	if since.IsZero() {
		t.Error("baseline StartedAt should be set")
	}
	if prog.PaidOff != 40000 || prog.Percent != 40 || prog.Remaining != 60000 {
		t.Errorf("progress = %+v, want paid 40000 / 40%% / rem 60000", prog)
	}

	if err := a.ClearPayoffTracking(); err != nil {
		t.Fatalf("ClearPayoffTracking: %v", err)
	}
	if _, _, tracking := a.PayoffProgress(60000); tracking {
		t.Error("tracking should be false after ClearPayoffTracking")
	}
}
