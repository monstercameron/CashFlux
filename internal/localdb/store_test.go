// SPDX-License-Identifier: MIT
package localdb_test

import (
	"github.com/monstercameron/CashFlux/internal/localdb"
	"github.com/monstercameron/CashFlux/internal/localdb/storetest"
	"testing"
)

func TestMemStoreConformance(t *testing.T) {
	storetest.Conformance(t, func() localdb.Store { return localdb.NewMemStore() })
}
