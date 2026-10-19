// Node-only extension of the shared IndexedDB fixture. This file is never
// loaded by production. Add standard bounded getAll/getAllKeys support without
// changing the localdb lane's fake. Ordinary unbounded calls keep their original
// behavior, including recovery-read tracing and injected request failures.
(function () {
  "use strict";
  if (!globalThis.__fakeIDB || !globalThis.process) throw new Error("BOOT2 requires the isolated Node fixture");
  class KeyRange {
    constructor(lower, upper, lowerOpen, upperOpen) {
      this.lower=lower; this.upper=upper;
      this.lowerOpen=!!lowerOpen; this.upperOpen=!!upperOpen;
    }
    includes(key) {
      return (this.lower===undefined || (this.lowerOpen ? key>this.lower : key>=this.lower)) &&
             (this.upper===undefined || (this.upperOpen ? key<this.upper : key<=this.upper));
    }
    static bound(lower, upper, lowerOpen, upperOpen) { return new KeyRange(lower,upper,lowerOpen,upperOpen); }
    static only(key) { return new KeyRange(key,key,false,false); }
    static lowerBound(key, open) { return new KeyRange(key,undefined,open,false); }
    static upperBound(key, open) { return new KeyRange(undefined,key,false,open); }
  }
  globalThis.IDBKeyRange=KeyRange;
  return new Promise((resolve,reject)=>{
    const req=indexedDB.open("cashflux-kv",1);
    req.onerror=()=>reject(req.error);
    req.onsuccess=()=>{
      const db=req.result, tx=db.transaction("kv","readonly"), store=tx.objectStore("kv");
      const proto=Object.getPrototypeOf(store);
      if (!Object.hasOwn(proto,"__boot2Ranges")) {
        for (const name of ["getAll","getAllKeys"]) {
          const original=proto[name];
          proto[name]=function(query,count) {
            if (!query) return original.call(this);
            const data=this._st;
            return this._tx._enqueue(false,()=>{
              const entries=Array.from(data.data.entries()).sort((a,b)=>a[0]<b[0]?-1:a[0]>b[0]?1:0)
                .filter(pair=>query.includes(pair[0]));
              const limited=count===undefined?entries:entries.slice(0,count);
              return limited.map(pair=>structuredClone(pair[name==="getAll"?1:0]));
            });
          };
        }
        Object.defineProperty(proto,"__boot2Ranges",{value:true});
      }
      tx.oncomplete=()=>{ db.close(); resolve(); };
      tx.onabort=()=>reject(tx.error);
    };
  });
})()
