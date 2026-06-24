// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package artifactstore

// OpenIDB is not available outside js/wasm. It always returns ErrUnavailable.
// This stub lets native-Go tests import the package without syscall/js.
func OpenIDB() (*IDBStore, error) { return nil, ErrUnavailable }

// IDBStore is the stub type on non-wasm platforms. None of its methods are
// reachable because OpenIDB always errors, but the type must exist so that
// code referencing *IDBStore compiles everywhere.
type IDBStore struct{}

func (s *IDBStore) Put(id string, mime string, data []byte) error {
	return ErrUnavailable
}
func (s *IDBStore) Get(id string) (string, []byte, bool, error) {
	return "", nil, false, ErrUnavailable
}
func (s *IDBStore) Delete(id string) error { return ErrUnavailable }
func (s *IDBStore) Usage() (int64, error)  { return 0, nil }
