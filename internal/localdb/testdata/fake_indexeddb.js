// Test-only in-memory IndexedDB shim for the Node-based wasm test lane.
//
// Node has no indexedDB, so idb_wasm_test.go embeds this file and evals it
// before running the shared Store conformance suite against IDBStore. It is
// never shipped with the app: production always uses the browser's real
// IndexedDB, and the test prefers a real implementation when one exists.
//
// Fidelity (the parts the adapter depends on):
//   - one named database is shared by every open() connection, so two
//     IDBStore instances in one test behave like two browser tabs;
//   - transactions run one at a time per database in creation order, which
//     is stronger than (and therefore compatible with) the spec rule that
//     overlapping readwrite transactions are serialized;
//   - a transaction snapshots its stores when it starts and rolls back to
//     that snapshot on abort or on any request error, so failed writes
//     publish nothing;
//   - unique indexes and add() primary-key conflicts raise ConstraintError;
//   - getAll/getAllKeys return entries in IndexedDB key order (numbers
//     ascending, then strings);
//   - errors are Error objects whose .name is the DOMException name.
//
// Known simplifications versus a real browser: no cursors, no versionchange
// blocking, no persistence, and transaction commit is decided a couple of
// milliseconds after the request queue drains (real browsers commit at the
// next microtask checkpoint; the grace period absorbs the Go wasm
// goroutine handoff between chained requests in one transaction).
//
// Test controls live on globalThis.__fakeIDB:
//   failNextRequest(name) - the next request errors with this DOM name
//   failNextWrite(name)   - the next put/add/delete errors with this name
//   failNextOpen(name)    - the next open() errors with this name
//   reset()               - drop every database and pending failure
//   dump(dbName)          - JSON string of {store: [[key, value], ...]}
(function () {
  "use strict";

  var databases = new Map();

  var control = {
    _failNext: null,
    _failNextWrite: null,
    _failNextOpen: null,
    _failNextTransaction: null,
    _transactionCount: 0,
    failNextRequest: function (name) { control._failNext = name; },
    failNextWrite: function (name) { control._failNextWrite = name; },
    failNextOpen: function (name) { control._failNextOpen = name; },
    failNextTransaction: function (name) { control._failNextTransaction = name; },
    transactionCount: function () { return control._transactionCount; },
    reset: function () {
      databases.clear();
      control._failNext = null;
      control._failNextWrite = null;
      control._failNextOpen = null;
      control._failNextTransaction = null;
      control._transactionCount = 0;
    },
    dump: function (name) {
      var db = databases.get(name);
      if (!db) return "{}";
      var out = {};
      db.stores.forEach(function (st, storeName) {
        out[storeName] = sortedEntries(st).map(function (pair) {
          return [String(pair[0]), pair[1]];
        });
      });
      return JSON.stringify(out);
    }
  };
  globalThis.__fakeIDB = control;

  function domError(name, message) {
    var e = new Error(message || name);
    e.name = name;
    return e;
  }

  function clone(v) {
    if (v === undefined) return undefined;
    if (typeof structuredClone === "function") return structuredClone(v);
    return JSON.parse(JSON.stringify(v));
  }

  function cmpKeys(a, b) {
    var ta = typeof a, tb = typeof b;
    if (ta === "number" && tb === "number") return a - b;
    if (ta === "number") return -1;
    if (tb === "number") return 1;
    var sa = String(a), sb = String(b);
    return sa < sb ? -1 : sa > sb ? 1 : 0;
  }

  function sortedEntries(st) {
    return Array.from(st.data.entries()).sort(function (x, y) {
      return cmpKeys(x[0], y[0]);
    });
  }

  function FakeRequest() {
    this.result = undefined;
    this.error = null;
    this.readyState = "pending";
    this.onsuccess = null;
    this.onerror = null;
    this.onupgradeneeded = null;
  }

  function fire(target, handler) {
    var cb = target[handler];
    if (cb) cb({ target: target, preventDefault: function () {} });
  }

  function StoreData() {
    this.data = new Map();
    this.indexes = new Map();
  }
  StoreData.prototype.snapshot = function () {
    var m = new Map();
    this.data.forEach(function (v, k) { m.set(clone(k), clone(v)); });
    return m;
  };

  function IndexHandle(tx, st, def) {
    this._tx = tx;
    this._st = st;
    this._def = def;
  }
  IndexHandle.prototype.get = function (key) {
    var st = this._st, def = this._def;
    return this._tx._enqueue(false, function () {
      var entries = sortedEntries(st);
      for (var i = 0; i < entries.length; i++) {
        var v = entries[i][1];
        if (v && typeof v === "object" && v[def.keyPath] === key) return clone(v);
      }
      return undefined;
    });
  };
  IndexHandle.prototype.getKey = function (key) {
    var st = this._st, def = this._def;
    return this._tx._enqueue(false, function () {
      var entries = sortedEntries(st);
      for (var i = 0; i < entries.length; i++) {
        var v = entries[i][1];
        if (v && typeof v === "object" && v[def.keyPath] === key) return clone(entries[i][0]);
      }
      return undefined;
    });
  };

  function StoreHandle(tx, name, st) {
    this._tx = tx;
    this.name = name;
    this._st = st;
  }
  StoreHandle.prototype._checkUnique = function (key, value) {
    var st = this._st;
    st.indexes.forEach(function (def) {
      if (!def.unique) return;
      var iv = value && typeof value === "object" ? value[def.keyPath] : undefined;
      if (iv === undefined) return;
      st.data.forEach(function (v, k) {
        if (k !== key && v && typeof v === "object" && v[def.keyPath] === iv) {
          throw domError("ConstraintError", "unique index on " + def.keyPath + " violated");
        }
      });
    });
  };
  StoreHandle.prototype.get = function (key) {
    var st = this._st;
    return this._tx._enqueue(false, function () { return clone(st.data.get(key)); });
  };
  StoreHandle.prototype.put = function (value, key) {
    var st = this._st, self = this;
    return this._tx._enqueue(true, function () {
      self._checkUnique(key, value);
      st.data.set(key, clone(value));
      return key;
    });
  };
  StoreHandle.prototype.add = function (value, key) {
    var st = this._st, self = this;
    return this._tx._enqueue(true, function () {
      if (st.data.has(key)) throw domError("ConstraintError", "key already exists: " + key);
      self._checkUnique(key, value);
      st.data.set(key, clone(value));
      return key;
    });
  };
  StoreHandle.prototype.delete = function (key) {
    var st = this._st;
    return this._tx._enqueue(true, function () { st.data.delete(key); return undefined; });
  };
  StoreHandle.prototype.clear = function () {
    var st = this._st;
    return this._tx._enqueue(true, function () { st.data.clear(); return undefined; });
  };
  StoreHandle.prototype.getAll = function () {
    var st = this._st;
    return this._tx._enqueue(false, function () {
      return sortedEntries(st).map(function (p) { return clone(p[1]); });
    });
  };
  StoreHandle.prototype.getAllKeys = function () {
    var st = this._st;
    return this._tx._enqueue(false, function () {
      return sortedEntries(st).map(function (p) { return clone(p[0]); });
    });
  };
  StoreHandle.prototype.createIndex = function (name, keyPath, opts) {
    var def = { keyPath: keyPath, unique: !!(opts && opts.unique) };
    this._st.indexes.set(name, def);
    return new IndexHandle(this._tx, this._st, def);
  };
  StoreHandle.prototype.index = function (name) {
    var def = this._st.indexes.get(name);
    if (!def) throw domError("NotFoundError", "no index named " + name);
    return new IndexHandle(this._tx, this._st, def);
  };

  function pumpDB(db) {
    if (db._activeTx) return;
    while (db._txQueue.length > 0) {
      var tx = db._txQueue.shift();
      if (tx._state === "waiting") {
        db._activeTx = tx;
        tx._activate();
        return;
      }
    }
  }

  function Transaction(db, names, mode) {
    this._db = db;
    this._names = names;
    this.mode = mode || "readonly";
    this.error = null;
    this.oncomplete = null;
    this.onerror = null;
    this.onabort = null;
    this._queue = [];
    this._state = "waiting"; // waiting -> active -> committed | aborted
    this._snapshot = null;
    this._pumping = false;
    this._commitTimer = null;
    db._txQueue.push(this);
    pumpDB(db);
  }
  Transaction.prototype.objectStore = function (name) {
    if (this._names.indexOf(name) === -1) {
      throw domError("NotFoundError", "store " + name + " is not in transaction scope");
    }
    var st = this._db.stores.get(name);
    if (!st) throw domError("NotFoundError", "no store named " + name);
    return new StoreHandle(this, name, st);
  };
  Transaction.prototype.abort = function () {
    var self = this;
    setTimeout(function () {
      self._doAbort(domError("AbortError", "transaction aborted"));
    }, 0);
  };
  Transaction.prototype._activate = function () {
    var db = this._db;
    this._state = "active";
    this._snapshot = new Map();
    var snap = this._snapshot;
    this._names.forEach(function (n) {
      var st = db.stores.get(n);
      if (st) snap.set(n, st.snapshot());
    });
    this._pump();
  };
  Transaction.prototype._enqueue = function (isWrite, fn) {
    if (this._state === "committed" || this._state === "aborted") {
      throw domError("TransactionInactiveError", "transaction is finished");
    }
    if (isWrite && this.mode === "readonly") {
      throw domError("ReadOnlyError", "transaction is readonly");
    }
    var req = new FakeRequest();
    this._queue.push({ req: req, fn: fn, isWrite: isWrite });
    this._clearCommitTimer();
    if (this._state === "active") this._pump();
    return req;
  };
  Transaction.prototype._pump = function () {
    if (this._pumping || this._state !== "active") return;
    this._pumping = true;
    var self = this;
    setTimeout(function () { self._step(); }, 0);
  };
  Transaction.prototype._step = function () {
    if (this._state !== "active") { this._pumping = false; return; }
    var item = this._queue.shift();
    if (!item) { this._pumping = false; this._scheduleCommit(); return; }
    var failName = null;
    if (control._failNext) { failName = control._failNext; control._failNext = null; }
    else if (item.isWrite && control._failNextWrite) {
      failName = control._failNextWrite; control._failNextWrite = null;
    }
    if (failName) {
      var injected = domError(failName, "injected failure: " + failName);
      item.req.error = injected;
      item.req.readyState = "done";
      fire(item.req, "onerror");
      this._pumping = false;
      this._doAbort(injected);
      return;
    }
    try {
      item.req.result = item.fn();
      item.req.readyState = "done";
      fire(item.req, "onsuccess");
    } catch (e) {
      item.req.error = e;
      item.req.readyState = "done";
      fire(item.req, "onerror");
      this._pumping = false;
      this._doAbort(e);
      return;
    }
    var self = this;
    setTimeout(function () { self._step(); }, 0);
  };
  Transaction.prototype._scheduleCommit = function () {
    this._clearCommitTimer();
    var self = this;
    this._commitTimer = setTimeout(function () {
      if (self._state === "active" && self._queue.length === 0) self._doCommit();
    }, 2);
  };
  Transaction.prototype._clearCommitTimer = function () {
    if (this._commitTimer) { clearTimeout(this._commitTimer); this._commitTimer = null; }
  };
  Transaction.prototype._doCommit = function () {
    if (this._state !== "active") return;
    this._state = "committed";
    this._snapshot = null;
    this._clearCommitTimer();
    this._finish();
    fire(this, "oncomplete");
  };
  Transaction.prototype._doAbort = function (err) {
    if (this._state === "committed" || this._state === "aborted") return;
    this.error = err;
    var db = this._db;
    if (this._snapshot) {
      this._snapshot.forEach(function (data, n) {
        var st = db.stores.get(n);
        if (st) st.data = data;
      });
      this._snapshot = null;
    }
    var qi = db._txQueue.indexOf(this);
    if (qi !== -1) db._txQueue.splice(qi, 1);
    this._state = "aborted";
    this._queue.length = 0;
    this._clearCommitTimer();
    var wasActive = db._activeTx === this;
    this._finish();
    // Programmatic abort of a not-yet-active transaction still settles it.
    if (wasActive || this.onerror || this.onabort) {
      fire(this, "onerror");
      fire(this, "onabort");
    }
  };
  Transaction.prototype._finish = function () {
    if (this._db._activeTx === this) {
      this._db._activeTx = null;
      pumpDB(this._db);
    }
  };

  function DBHandle(name, data) {
    this.name = name;
    this._data = data;
    this.version = data.version;
    // Real IndexedDB exposes objectStoreNames as a DOMStringList property.
    // A function object with a contains() method satisfies both property
    // access (current adapter) and accidental call access (older code).
    var self = this;
    var namesFn = function () { return self._namesList(); };
    namesFn.contains = function (n) { return self._data.stores.has(n); };
    this.objectStoreNames = namesFn;
  }
  DBHandle.prototype._namesList = function () {
    var names = Array.from(this._data.stores.keys());
    return {
      length: names.length,
      contains: function (n) { return names.indexOf(n) !== -1; },
      item: function (i) { return i < names.length ? names[i] : null; }
    };
  };
  DBHandle.prototype.createObjectStore = function (name, opts) {
    var st = this._data.stores.get(name);
    if (!st) { st = new StoreData(); this._data.stores.set(name, st); }
    return new StoreHandle(null, name, st);
  };
  DBHandle.prototype.deleteObjectStore = function (name) {
    this._data.stores.delete(name);
  };
  DBHandle.prototype.transaction = function (names, mode, opts) {
    control._transactionCount++;
    if (control._failNextTransaction) {
      var name = control._failNextTransaction;
      control._failNextTransaction = null;
      throw domError(name, "injected transaction failure");
    }
    var arr = Array.isArray(names) ? names.slice() : [names];
    return new Transaction(this._data, arr, mode);
  };
  DBHandle.prototype.close = function () {};

  var factory = {
    open: function (name, version) {
      var req = new FakeRequest();
      setTimeout(function () {
        if (control._failNextOpen) {
          var n = control._failNextOpen;
          control._failNextOpen = null;
          req.error = domError(n, "injected open failure: " + n);
          req.readyState = "done";
          fire(req, "onerror");
          return;
        }
        var data = databases.get(name);
        var isNew = !data;
        if (!data) {
          data = { version: version || 1, stores: new Map(), _txQueue: [], _activeTx: null };
          databases.set(name, data);
        }
        var db = new DBHandle(name, data);
        req.result = db;
        if (isNew || (version && version > data.version)) {
          if (version) data.version = version;
          fire(req, "onupgradeneeded");
        }
        req.readyState = "done";
        fire(req, "onsuccess");
      }, 0);
      return req;
    },
    deleteDatabase: function (name) {
      var req = new FakeRequest();
      setTimeout(function () {
        databases.delete(name);
        req.readyState = "done";
        fire(req, "onsuccess");
      }, 0);
      return req;
    }
  };

  globalThis.indexedDB = factory;
})();
