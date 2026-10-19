package syncstate

import "testing"

func TestDecideEncryptedUpload(t *testing.T) {
	tests := []struct {
		name            string
		encryptionReady bool
		wantAllowed     bool
		wantStatus      EncryptionStatus
		wantMessage     string
	}{
		{name: "no passcode or key refuses", wantStatus: StatusNeedsDataProtection, wantMessage: DataProtectionMessage},
		{name: "encryption ready permits ciphertext", encryptionReady: true, wantAllowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecideEncryptedUpload(tt.encryptionReady)
			if got.Allowed != tt.wantAllowed || got.Status != tt.wantStatus || got.Message != tt.wantMessage {
				t.Fatalf("DecideEncryptedUpload(%t) = %+v", tt.encryptionReady, got)
			}
		})
	}
}
