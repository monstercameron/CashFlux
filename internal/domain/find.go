package domain

// AccountByID returns the account with the given id and whether it was found. A
// linear scan, which is appropriate for the household-scale account lists CashFlux
// works with (and keeps callers from re-implementing the lookup).
func AccountByID(accounts []Account, id string) (Account, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return Account{}, false
}
