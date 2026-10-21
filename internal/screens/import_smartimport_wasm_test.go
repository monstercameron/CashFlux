// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestSmartImportServerStatementReachesReview(t *testing.T) {
	fakeImportFetch(t)
	f, current, dialog := mountImportControllerSurface(t, true)
	c := current()
	settings := c.app.Settings()
	settings.OpenAIKey = ""
	if err := c.app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	c.chooseSource("stmt")
	c.stmtText.Set("2026-03-01 First purchase -1.25")
	f.Stabilize()
	c = current()
	c.useBackendAI = true
	c.pr.ServerURL = "https://example.invalid"
	c.pr.ServerToken = "fake-token"
	previous := importProxyStatementChat
	t.Cleanup(func() { importProxyStatementChat = previous })
	var reply func(string, ai.Usage)
	sends := 0
	importProxyStatementChat = func(endpoint, token, model string, messages []ai.Message, temperature float64, result func(string, ai.Usage), fail func(string)) func() {
		sends++
		if endpoint != "https://example.invalid" || token != "fake-token" || len(messages) != 2 || !strings.Contains(messages[0].Content, `"transactions"`) {
			t.Fatalf("incorrect proxy request: endpoint=%q token=%q messages=%+v", endpoint, token, messages)
		}
		reply = result
		return func() {}
	}
	c.requestExtraction("stmt")
	if sends != 0 || dialog() == nil || !strings.Contains(dialog().Message, c.pr.ServerURL) {
		t.Fatalf("missing proxy consent; status=%q", f.Text())
	}
	dialog().OnResult(true, "")
	if sends != 1 || reply == nil {
		t.Fatalf("server statement sends=%d", sends)
	}
	f.Stabilize()
	reply(smartImportExtraction, ai.Usage{TotalTokens: 42})
	f.Stabilize()
	if len(current().model.Rows) != 3 || findByTestID(f, "div", "draft-review-page") == nil {
		t.Fatalf("server statement has no review: %s", f.Text())
	}
	smartImportAccountPicker(t, f).Change("acct-2")
	f.Stabilize()
	if current().importAcct.Get() != "acct-2" {
		t.Fatal("account placement did not change")
	}
	smartImportCommitAndCheck(t, f, current, 3)
}

func TestSmartImportStaleConsentCannotChangeCurrentJob(t *testing.T) {
	for _, oldChoice := range []bool{false, true} {
		name := "decline"
		if oldChoice {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			f, current, dialog := mountImportControllerSurface(t, true)
			c := current()
			c.chooseSource("stmt")
			sends := 0
			var reply func(string, ai.Usage)
			send := func(result func(string, ai.Usage), _ func(string)) func() {
				sends++
				reply = result
				return func() {}
			}
			c.startAI(send, c.acceptExtraction)
			old := dialog()
			c.chooseSource("pdf")
			c.startAI(send, c.acceptExtraction)
			next := dialog()
			old.OnResult(oldChoice, "")
			if !c.aiLoading.Get() || c.job.consented || sends != 0 {
				t.Fatal("stale consent changed or authorized the current job")
			}
			next.OnResult(true, "")
			if sends != 1 || reply == nil {
				t.Fatalf("current extraction sends=%d", sends)
			}
			reply(smartImportExtraction, ai.Usage{})
			f.Stabilize()
			if len(current().model.Rows) != 3 || findByTestID(f, "div", "draft-review-page") == nil {
				t.Fatal("current extraction lost its review")
			}
		})
	}
}

const smartImportExtraction = `{"transactions":[
	{"date":"2026-03-01","description":"First purchase","amount":"-1.25","category":"Groceries","account_id":"","currency":"USD"},
	{"date":"2026-03-02","description":"Second purchase","amount":"-2.50","category":"Groceries","account_id":"","currency":"USD"},
	{"date":"2026-03-03","description":"Third purchase","amount":"-3.75","category":"Groceries","account_id":"","currency":"USD"}]}`

// fakeImportFetch replaces the entire fetch boundary; even an unexpected URL
// stays offline. Replies remain pending until the test explicitly delivers them.
func fakeImportFetch(t *testing.T) js.Value {
	t.Helper()
	previous := js.Global().Get("fetch")
	fake := js.Global().Get("Function").New(`
		const calls = [];
		globalThis.fetch = (url, options) => new Promise(resolve => {
			calls.push({url, options, reply: body => resolve({status:200, text:() => Promise.resolve(body)})});
		});
		return calls;
	`).Invoke()
	t.Cleanup(func() { js.Global().Set("fetch", previous) })
	return fake
}

func deliverImportExtraction(t *testing.T, calls js.Value, index int) {
	t.Helper()
	deliverImportContent(t, calls, index, smartImportExtraction)
}

func deliverImportContent(t *testing.T, calls js.Value, index int, content string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}, "usage": map[string]any{"total_tokens": 42}})
	if err != nil {
		t.Fatal(err)
	}
	calls.Index(index).Call("reply", string(body))
	// Let the real transport's fetch/text promises drain before inspecting Go
	// state. No network or wall-clock ordering participates in the result.
	time.Sleep(10 * time.Millisecond)
}

func TestSmartImportReviewAndAccountPlacement(t *testing.T) {
	for _, source := range []string{"local-statement", "stmt", "pdf", "image-statement", "receipt"} {
		t.Run(source, func(t *testing.T) {
			calls := fakeImportFetch(t)
			f, current, dialog := mountImportControllerSurface(t, true)
			pick := "stmt"
			if source == "pdf" {
				pick = "pdf"
			}
			if source == "image-statement" || source == "receipt" {
				pick = "receipt"
			}
			findByTestID(f, "button", "import-type-"+pick).Click()
			f.Stabilize()
			if pick == "stmt" {
				f.AllByTag("textarea")[0].Input("date,description,amount\n2026-03-01,First purchase,-1.25\n2026-03-02,Second purchase,-2.50\n2026-03-03,Third purchase,-3.75")
			} else if pick == "pdf" {
				current().pdfData.Set("data:application/pdf;base64,JVBERi0=")
				current().pdfName.Set("statement.pdf")
			} else {
				current().imageURL.Set("data:image/png;base64,aW1hZ2U=")
			}
			f.Stabilize()
			if source == "local-statement" {
				for _, form := range f.AllByTag("form") {
					if form.Attr("id") != ImportModalFormID {
						form.Submit()
						break
					}
				}
			} else {
				if pick == "stmt" {
					findByTestID(f, "button", "extract-ai-btn").Click()
				}
				if pick == "pdf" {
					findByTestID(f, "button", "statementimport-run").Click()
				}
				if pick == "receipt" {
					for _, button := range f.AllByTag("button") {
						if strings.Contains(button.Text(), uistate.T("documents.readAI")) {
							button.Click()
							break
						}
					}
				}
				if calls.Length() != 0 || dialog() == nil {
					t.Fatal("missing consent or sent before consent")
				}
				dialog().OnResult(true, "")
				f.Stabilize()
				if calls.Length() != 1 {
					t.Fatalf("fake requests = %d; status=%q", calls.Length(), f.Text())
				}
				content := smartImportExtraction
				if source == "receipt" {
					content = strings.ReplaceAll(content, `"amount":"-`, `"amount":"`)
				}
				deliverImportContent(t, calls, 0, content)
				// Fetch and response.text settle on separate JavaScript turns. Wait for
				// the actual review state before this fixture is torn down; a single
				// 10 ms yield can leave the response callback running into the next test.
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					f.Stabilize()
					c := current()
					if c.stage.Get() == "review" && c.inReview && len(c.model.Rows) == 3 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			f.Stabilize()
			c := current()
			if c.stage.Get() != "review" || !c.inReview || len(c.model.Rows) != 3 {
				t.Fatalf("stage=%q review=%v rows=%d status=%q", c.stage.Get(), c.inReview, len(c.model.Rows), f.Text())
			}
			page := findByTestID(f, "div", "draft-review-page")
			if page == nil {
				t.Fatal("review list is missing")
			}
			for _, text := range []string{"First purchase", "Second purchase", "Third purchase", "1.25", "2.50", "3.75"} {
				if !strings.Contains(page.Text(), text) {
					t.Fatalf("review missing %q: %s", text, page.Text())
				}
			}
			for _, row := range c.model.Rows {
				if !strings.Contains(page.Text(), row.Date) {
					t.Fatalf("review missing date %q: %s", row.Date, page.Text())
				}
			}
			if source == "receipt" {
				for _, toggle := range f.AllByTag("div") {
					if toggle.Attr("role") == "switch" && toggle.Attr("aria-label") == uistate.T("documents.receiptToggle") {
						toggle.Click()
						break
					}
				}
				f.Stabilize()
				if !current().receiptMode.Get() {
					t.Fatal("receipt split mode could not be selected")
				}
			}
			picker := smartImportAccountPicker(t, f)
			picker.Change("acct-2")
			f.Stabilize()
			if current().importAcct.Get() != "acct-2" {
				t.Fatal("account picker did not change")
			}
			count := 3
			if source == "receipt" {
				count = 1
			}
			smartImportCommitAndCheck(t, f, current, count)
		})
	}
}

func smartImportCommitAndCheck(t *testing.T, f *render.Fixture, current func() *documentsController, count int) {
	t.Helper()
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) { done(nil) })
	defer restore()
	var saved func(appstate.DurableResult)
	current().app.SetDurableWriter(func(_ uint64, done func(appstate.DurableResult)) { saved = done })
	f.AllByTag("form")[0].Submit()
	f.Stabilize()
	if saved == nil || !current().busy() || len(current().model.Rows) != 3 {
		t.Fatalf("import acknowledged before durable save; %s", f.Text())
	}
	saved(appstate.DurableResult{Revision: current().app.Rev()})
	f.Stabilize()
	txns := current().app.Transactions()
	if len(txns) != count || len(current().model.Rows) != 0 || current().busy() {
		t.Fatalf("committed %d transactions; %s", len(txns), f.Text())
	}
	for _, tx := range txns {
		if tx.AccountID != "acct-2" {
			t.Fatalf("transaction placed in %q", tx.AccountID)
		}
		if count == 1 && (tx.Amount.Amount != -750 || len(tx.Splits) == 0) {
			t.Fatalf("receipt was not imported as a split purchase: %+v", tx)
		}
	}
}

func smartImportAccountPicker(t *testing.T, f *render.Fixture) *render.QueryNode {
	t.Helper()
	for _, picker := range f.AllByTag("select") {
		if picker.Attr("aria-label") == uistate.T("documents.importAccount") {
			return picker
		}
	}
	t.Fatal("review account picker is missing")
	return nil
}
