// SPDX-License-Identifier: MIT

package backendauth

import "testing"

func TestValidateServerEndpoint(t *testing.T) {
	tests := []struct {
		address string
		ok      bool
	}{
		{"https://budget.earlcameron.com", true},
		{"wss://sync.example.com/grpc", true},
		{"http://127.0.0.1:8081", true},
		{"ws://localhost:8081/grpc", true},
		{"http://[::1]:8081", true},
		{"http://127.0.0.2:8081", true},
		{"http://192.168.1.10:8081", false},
		{"ws://sync.example.com/grpc", false},
		{"http://localhost.evil.example", false},
		{"https://user:pass@sync.example.com", false},
		{"https://sync.example.com?token=secret", false},
		{"https://sync.example.com/#fragment", false},
		{"ftp://sync.example.com", false},
		{"sync.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if err := ValidateServerEndpoint(tt.address); (err == nil) != tt.ok {
				t.Fatalf("ValidateServerEndpoint(%q) error = %v, want success %t", tt.address, err, tt.ok)
			}
		})
	}
}
