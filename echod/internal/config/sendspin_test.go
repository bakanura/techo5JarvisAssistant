package config

import "testing"

func TestSendspinSecureDefault(t *testing.T) {
	got := defaultSendspin()
	if got.Enabled {
		t.Fatal("Sendspin is enabled by default")
	}
	if got.ServerIP != "" {
		t.Fatalf("Sendspin default server = %q, want unpaired", got.ServerIP)
	}
}
