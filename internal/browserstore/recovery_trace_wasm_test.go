//go:build js && wasm

package browserstore

import (
	"fmt"
	"strings"
	"syscall/js"
	"testing"
)

// SeedRecoveryForTest installs synthetic opaque recovery payloads without using
// production retention, emulating an old oversized installation. No real browser
// data is accessed by the Node IndexedDB shim.
func SeedRecoveryForTest(t *testing.T, stale, quarantine int) {
	t.Helper()
	var seeded []string
	for i := 0; i < stale; i++ {
		seeded = append(seeded, fmt.Sprintf("cashflux:stale-recovery:legacy:%04d", i))
	}
	for i := 0; i < quarantine; i++ {
		seeded = append(seeded, fmt.Sprintf("cashflux:quarantine:%d", 1700000000+i))
	}
	seeded = append(seeded, "cashflux:journal-recovery:legacy", "cashflux:dataset:checkpoint-backup")
	previous, err := readSecureBatch(seeded)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := recordTransaction(true, func(tx js.Value, _ func(js.Func), _ func(error)) {
			os := tx.Call("objectStore", idbStoreName)
			for _, key := range seeded {
				if raw, have := previous[key]; have {
					os.Call("put", raw, key)
				} else {
					os.Call("delete", key)
				}
			}
		})
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		for _, key := range seeded {
			if _, have := previous[key]; !have {
				delete(cache, key)
			}
		}
		mu.Unlock()
	})
	large := `{"format":"cashflux-atrest-v1","opaque":"` + strings.Repeat("X", 256*1024) + `"}`
	if err := recordTransaction(true, func(tx js.Value, _ func(js.Func), _ func(error)) {
		os := tx.Call("objectStore", idbStoreName)
		for i := 0; i < stale; i++ {
			os.Call("put", large, fmt.Sprintf("cashflux:stale-recovery:legacy:%04d", i))
		}
		for i := 0; i < quarantine; i++ {
			os.Call("put", large, fmt.Sprintf("cashflux:quarantine:%d", 1700000000+i))
		}
		os.Call("put", large, "cashflux:journal-recovery:legacy")
		os.Call("put", large, "cashflux:dataset:checkpoint-backup")
	}); err != nil {
		t.Fatal(err)
	}
}

// TraceRecoveryReadsForTest counts IndexedDB value reads, including getAll, at
// the actual object-store boundary. Enumeration of key names is not counted.
func TraceRecoveryReadsForTest(t *testing.T) func() (int, int) {
	t.Helper()
	originalDB, originalIDB := db, js.Global().Get("indexedDB")
	tracer := js.Global().Get("Function").New("idb", "db", `
 const stats={values:0,all:0};
 const recovery=k=>/^cashflux:(stale-recovery:|quarantine:|journal-recovery:|dataset:checkpoint-backup$)/.test(k);
 const os=s=>new Proxy(s,{get(t,k){
   if(k==="get") return key=>{if(recovery(key))stats.values++;return t.get(key)};
   if(k==="getAll") return (...a)=>{stats.all++;throw new Error("BOOT must not clone all recovery values")};
   const v=t[k]; return typeof v==="function"?v.bind(t):v;
 }});
 const tx=x=>new Proxy(x,{get(t,k){if(k==="objectStore")return (...a)=>os(t.objectStore(...a));const v=t[k];return typeof v==="function"?v.bind(t):v},set(t,k,v){t[k]=v;return true}});
 const database=x=>new Proxy(x,{get(t,k){if(k==="transaction")return (...a)=>tx(t.transaction(...a));const v=t[k];return k!=="objectStoreNames"&&typeof v==="function"?v.bind(t):v}});
 const indexed=new Proxy(idb,{get(t,k){if(k==="open")return (...a)=>new Proxy(t.open(...a),{get(r,p){return p==="result"&&r.result?database(r.result):r[p]},set(r,p,v){r[p]=v;return true}});const v=t[k];return typeof v==="function"?v.bind(t):v}});
 return {stats,indexed,db:database(db)};
 `).Invoke(originalIDB, originalDB)
	db = tracer.Get("db")
	js.Global().Set("indexedDB", tracer.Get("indexed"))
	t.Cleanup(func() { db = originalDB; js.Global().Set("indexedDB", originalIDB) })
	return func() (int, int) { s := tracer.Get("stats"); return s.Get("values").Int(), s.Get("all").Int() }
}

func TestBOOTTwoHundredLargeRecoveryValuesAreNotRead(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "live checkpoint")
	SeedRecoveryForTest(t, 200, 8)
	reads := TraceRecoveryReadsForTest(t)
	ready = false
	secureInitErr = nil
	Init()
	// A second instance's recovery notifications must also remain key-only.
	cold := []string{"cashflux:stale-recovery:legacy:0000", "cashflux:quarantine:1700000000", "cashflux:journal-recovery:legacy", "cashflux:dataset:checkpoint-backup"}
	Reload(cold...)
	reloadPublishedBatch(cold)
	values, all := reads()
	if values != 0 || all != 0 {
		t.Fatalf("BOOT read recovery values=%d, getAll scans=%d; want zero", values, all)
	}
	if err := InitError(); err != nil {
		t.Fatal(err)
	}
	if GetString("cashflux:dataset") != "live checkpoint" {
		t.Fatal("active checkpoint missing")
	}
	count := 0
	for _, key := range Keys() {
		if strings.HasPrefix(key, "cashflux:stale-recovery:") {
			count++
		}
	}
	if count != 200 {
		t.Fatalf("recovery names=%d, want 200", count)
	}
}
