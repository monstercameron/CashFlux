//go:build js && wasm

package browserstore

import (
	"fmt"
	"sort"
	"strings"
	"syscall/js"
)

// readRecordRequests keeps every read in the caller's transaction. Large sets of
// journal conditions use two bounded requests per workspace, rather than one
// browser event per tombstone. Recovery families and other workspaces are never
// included in these value ranges. Small selections use ordinary get requests.
// Callbacks run synchronously inside request events, so conditional puts remain
// in the same active strict transaction as their ciphertext comparisons.
func readRecordRequests(tx js.Value, keys []string, retain func(js.Func), fail func(error), read func(string, js.Value) bool) {
	os := tx.Call("objectStore", idbStoreName)
	guarded := func(fn func()) {
		defer func() {
			if r := recover(); r != nil {
				fail(jsPanicError("read protected batch", r))
				tx.Call("abort")
			}
		}()
		fn()
	}
	groups := map[string][]string{}
	var singles []string
	ranges := js.Global().Get("IDBKeyRange")
	for _, key := range keys {
		const journal = "cashflux:dataset:record:"
		workspace, _, have := strings.Cut(strings.TrimPrefix(key, journal), ":")
		if ranges.Truthy() && strings.HasPrefix(key, journal) && have && workspace != "" {
			prefix := journal + workspace + ":"
			groups[prefix] = append(groups[prefix], key)
		} else {
			singles = append(singles, key)
		}
	}
	var prefixes []string
	for prefix, selected := range groups {
		if len(selected) < 16 {
			singles = append(singles, selected...)
		} else {
			prefixes = append(prefixes, prefix)
		}
	}
	sort.Strings(prefixes)
	sort.Strings(singles)
	for _, key := range singles {
		req := os.Call("get", key)
		callback := js.FuncOf(func(js.Value, []js.Value) any { guarded(func() { read(key, req.Get("result")) }); return nil })
		retain(callback)
		req.Set("onsuccess", callback)
	}
	for _, prefix := range prefixes {
		selected := groups[prefix]
		// The prefix ends in ':', so ';' is its exclusive lexicographic successor,
		// including even journal IDs whose suffix contains U+FFFF.
		query := ranges.Call("bound", prefix, prefix[:len(prefix)-1]+";", false, true)
		names := os.Call("getAllKeys", query)
		values := os.Call("getAll", query)
		namesReady, valuesReady := false, false
		emit := func() {
			if !namesReady || !valuesReady {
				return
			}
			n, v := names.Get("result"), values.Get("result")
			if n.Length() != v.Length() {
				fail(fmt.Errorf("journal range %q returned mismatched keys and values", prefix))
				tx.Call("abort")
				return
			}
			found := make(map[string]js.Value, n.Length())
			for i := 0; i < n.Length(); i++ {
				found[n.Index(i).String()] = v.Index(i)
			}
			for _, key := range selected {
				value, have := found[key]
				if !have {
					value = js.Undefined()
				}
				if !read(key, value) {
					return
				}
			}
		}
		onNames := js.FuncOf(func(js.Value, []js.Value) any { guarded(func() { namesReady = true; emit() }); return nil })
		onValues := js.FuncOf(func(js.Value, []js.Value) any { guarded(func() { valuesReady = true; emit() }); return nil })
		retain(onNames)
		retain(onValues)
		names.Set("onsuccess", onNames)
		values.Set("onsuccess", onValues)
	}
}
