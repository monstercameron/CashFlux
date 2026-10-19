//go:build !js || !wasm

package atrest

import "github.com/monstercameron/CashFlux/internal/keyring"

func platformDeriver() keyring.PBKDF2Deriver { return keyring.StandardPBKDF2{} }
