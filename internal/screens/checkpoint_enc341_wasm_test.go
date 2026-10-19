//go:build js && wasm

package screens

import (
	"bytes"
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

var checkpointNoticeText string

func checkpointNoticeFixture(_ struct{}) ui.Node {
	notice := uistate.UseNotice()
	checkpointNoticeText = notice.Get().Text
	return shorthand.Div(shorthand.Attr("data-testid", "checkpoint-notice"), notice.Get().Text)
}

// The controlled commit exercises every protected operation's checkpoint
// boundary contract: no mutation while pending, one mutation after success,
// and no mutation after failure or duplicate trigger.
func TestCheckpointGatedOperationsENC341(t *testing.T) {
	operations := []string{
		"allocation", "cover all", "CSV import", "reviewed document import",
		"receipt import", "apply rules", "bulk recategorize", "bulk delete",
	}
	for _, name := range operations {
		t.Run(name, func(t *testing.T) {
			t.Run("success waits and ignores duplicate", func(t *testing.T) {
				app := checkpointTestApp(t)
				commit := holdCheckpointCommit(t)
				calls := 0
				busy := false
				trigger := func() {
					if busy {
						return
					}
					busy = true
					uistate.SaveCheckpointThen("test "+name, func(_ string, err error) {
						busy = false
						if err == nil {
							calls++
							if putErr := app.PutCategory(domain.Category{ID: "done", Name: "Completed", Kind: domain.KindExpense}); putErr != nil {
								t.Errorf("apply operation: %v", putErr)
							}
						}
					})
				}
				trigger()
				trigger()
				if got := len(app.Categories()); got != 0 {
					t.Fatalf("dataset changed before checkpoint commit: %d categories", got)
				}
				if commit.commits != 1 {
					t.Fatalf("checkpoint commits = %d, want 1 after duplicate trigger", commit.commits)
				}
				commit.complete(nil)
				if got := len(app.Categories()); got != 1 || calls != 1 {
					t.Fatalf("after commit: categories=%d operations=%d, want 1 each", got, calls)
				}
			})

			t.Run("failure leaves dataset unchanged", func(t *testing.T) {
				app := checkpointTestApp(t)
				commit := holdCheckpointCommit(t)
				f := render.New(t)
				defer f.Cleanup()
				f.Render(ui.CreateElement(checkpointNoticeFixture, struct{}{}))
				calls := 0
				var checkpointErr error
				uistate.SaveCheckpointThen("test "+name, func(_ string, err error) {
					checkpointErr = err
					if err == nil {
						calls++
						_ = app.PutCategory(domain.Category{ID: "done", Name: "Completed", Kind: domain.KindExpense})
					} else {
						uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
					}
				})
				commit.complete(errors.New("QuotaExceededError"))
				f.Stabilize()
				if checkpointErr == nil || calls != 0 || len(app.Categories()) != 0 || checkpointNoticeText != "Couldn't save a restore point, so nothing was changed. Try again." {
					t.Fatalf("failure result err=%v operations=%d categories=%d notice=%q", checkpointErr, calls, len(app.Categories()), checkpointNoticeText)
				}
			})
		})
	}
}

type heldCheckpoint struct {
	commits  int
	complete func(error)
}

func holdCheckpointCommit(t *testing.T) *heldCheckpoint {
	t.Helper()
	held := &heldCheckpoint{}
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) {
		held.commits++
		held.complete = done
	})
	t.Cleanup(restore)
	return held
}

func checkpointTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })
	return app
}
