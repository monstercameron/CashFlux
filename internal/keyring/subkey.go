// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"
)

// hkdfSHA256 derives a 32-byte key from secret with HKDF-SHA-256 and the
// given info label. A nil salt is used: every input here is already a
// uniformly random or PRF-derived value, and the info label provides domain
// separation.
func hkdfSHA256(secret []byte, info string) ([]byte, error) {
	out, err := hkdf.Key(sha256.New, secret, nil, info, KeySize)
	if err != nil {
		return nil, fmt.Errorf("keyring: hkdf %q: %w", info, err)
	}
	return out, nil
}

// DeriveSubkey derives the 32-byte subkey for the given HKDF info label
// (one of SubkeyData, SubkeyAttachments, SubkeyAttachMAC, SubkeyVault).
// Subkeys are deterministic for a given WDK and label; distinct labels give
// independent keys, so one subkey leaking does not expose the others or the
// WDK. Zero the result when done with it.
func (wk *WorkspaceKey) DeriveSubkey(info string) ([]byte, error) {
	if wk.Destroyed() {
		return nil, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if info == "" {
		return nil, fmt.Errorf("%w: subkey info label is empty", ErrInvalidKey)
	}
	return hkdfSHA256(wk.material, info)
}

// DataKey returns the subkey that seals workspace data (changes/snapshots).
func (wk *WorkspaceKey) DataKey() ([]byte, error) { return wk.DeriveSubkey(SubkeyData) }

// AttachmentsKey returns the subkey that seals attachment contents.
func (wk *WorkspaceKey) AttachmentsKey() ([]byte, error) { return wk.DeriveSubkey(SubkeyAttachments) }

// AttachMACKey returns the subkey that derives attachment content addresses.
func (wk *WorkspaceKey) AttachMACKey() ([]byte, error) { return wk.DeriveSubkey(SubkeyAttachMAC) }

// VaultKey returns the subkey that seals stored credentials.
func (wk *WorkspaceKey) VaultKey() ([]byte, error) { return wk.DeriveSubkey(SubkeyVault) }

// SubkeyInfos returns every HKDF info label used for workspace subkeys, in a
// stable order.
func SubkeyInfos() []string {
	return []string{SubkeyData, SubkeyAttachments, SubkeyAttachMAC, SubkeyVault}
}
