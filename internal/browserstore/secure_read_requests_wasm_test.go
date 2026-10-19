//go:build js && wasm

package browserstore

import (
	"fmt"
	"strings"
	"syscall/js"
	"testing"
)

func TestBOOT2BoundedJournalReadsExcludeRecoveryAndOtherWorkspaces(t *testing.T) {
	initFixture(t)
	boot2FixtureRanges(t)
	values := map[string]string{}
	const prefix = "cashflux:dataset:record:boot2:"
	for i := range 32 {
		values[fmt.Sprintf("%s%02d", prefix, i)] = ""
	}
	values[prefix+"\uffff:last"] = "" // prefix upper bound must include this suffix
	const other = "cashflux:dataset:record:boot2-other:unrelated"
	const recovery = "cashflux:stale-recovery:boot2"
	values[other], values[recovery] = "other household", "retained recovery"
	if err := jrnlBatch(t, values, nil); err != nil {
		t.Fatal(err)
	}
	original := db
	traced := js.Global().Get("Function").New("db", "prefix", `
  const stats={ranges:0,forbidden:0};
  const wrap=store=>new Proxy(store,{get(t,k){
    if(k==="getAll")return query=>{
      if(!query || query.lower!==prefix || !query.upperOpen || query.upper!==prefix.slice(0,-1)+";" || query.includes("cashflux:stale-recovery:boot2") || query.includes("cashflux:dataset:record:boot2-other:unrelated")){
        stats.forbidden++;throw new Error("unbounded or foreign journal value read");
      }
      stats.ranges++;return t.getAll(query);
    };
    if(k==="get")return key=>{if(key.startsWith("cashflux:stale-recovery:")||key.startsWith("cashflux:dataset:record:boot2-other:")){stats.forbidden++;throw new Error("foreign value read")};return t.get(key)};
    const value=t[k];return typeof value==="function"?value.bind(t):value;
  }});
  const database=new Proxy(db,{get(t,k){
    if(k==="transaction")return (...args)=>{const tx=t.transaction(...args);const objectStore=tx.objectStore.bind(tx);tx.objectStore=(...args)=>wrap(objectStore(...args));return tx};
    const value=t[k];return typeof value==="function"?value.bind(t):value;
  }});
  return {database,stats};`).Invoke(original, prefix)
	db = traced.Get("database")
	defer func() { db = original }()
	snapshot, err := ReadSecureSnapshot(nil, []string{prefix})
	if err != nil || len(snapshot.Values) != 33 || len(snapshot.Errors) != 0 {
		t.Fatalf("bounded snapshot values=%d errors=%v err=%v", len(snapshot.Values), snapshot.Errors, err)
	}
	if _, have := snapshot.Values[other]; have {
		t.Fatal("snapshot mixed workspaces")
	}
	if _, have := snapshot.Values[recovery]; have {
		t.Fatal("snapshot read recovery values")
	}
	conditions := make([]string, 0, len(snapshot.Values)+1)
	for key := range snapshot.Values {
		conditions = append(conditions, key)
	}
	conditions = append(conditions, prefix+"absent")
	done := make(chan error, 1)
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	SetSecureSnapshotBatch(map[string]string{"cashflux:dataset": "repair", prefix + "absent": "new row"}, snapshot, conditions, func(err error) { done <- err })
	if err := waitResult(t, done); err == nil || !strings.Contains(err.Error(), "QuotaExceededError") {
		t.Fatalf("aborted bounded repair=%v", err)
	}
	if _, have, err := GetSecure("cashflux:dataset"); err != nil || have {
		t.Fatalf("failed repair published checkpoint: %v %v", have, err)
	}
	SetSecureSnapshotBatch(map[string]string{"cashflux:dataset": "repair", prefix + "absent": "new row"}, snapshot, conditions, func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	if got, _, err := GetSecure(prefix + "absent"); err != nil || got != "new row" {
		t.Fatalf("missing-key compare/write=%q %v", got, err)
	}
	if traced.Get("stats").Get("forbidden").Int() != 0 || traced.Get("stats").Get("ranges").Int() != 3 {
		t.Fatalf("range stats=%v", traced.Get("stats"))
	}
	// Older engines without the optional range interface retain the same result
	// and ciphertext checks using individual requests.
	rangeAPI := js.Global().Get("IDBKeyRange")
	js.Global().Set("IDBKeyRange", js.Undefined())
	fallback, err := ReadSecureSnapshot(nil, []string{prefix})
	js.Global().Set("IDBKeyRange", rangeAPI)
	if err != nil || fallback.Values[prefix+"absent"] != "new row" || len(fallback.Values) != 34 {
		t.Fatalf("range fallback values=%d err=%v", len(fallback.Values), err)
	}
}
