package config

import "testing"

func TestJarvisShowNetworkDefaultsFailClosed(t *testing.T) {
	st := load(t)
	got := st.Get()
	if got.Sendspin.Enabled {
		t.Fatal("sendspin is enabled before a Music Assistant server is paired")
	}
	if got.Sendspin.ServerIP != "" {
		t.Fatalf("sendspin server = %q on a fresh device", got.Sendspin.ServerIP)
	}
	if got.Diag.RemoteADB {
		t.Fatal("remote adb is enabled on a fresh device")
	}
	if got.Diag.InsecureTLS {
		t.Fatal("certificate verification is disabled on a fresh device")
	}
}

func TestLegacyDangerousDiagnosticBitsCanOnlyBeCleared(t *testing.T) {
	st := load(t)
	if err := st.Update(func(c *Config) {
		c.Diag.RemoteADB = true
		c.Diag.InsecureTLS = true
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Set().Diag().ClearLegacyRemoteADB(); err != nil {
		t.Fatal(err)
	}
	if err := st.Set().Diag().ClearLegacyInsecureTLS(); err != nil {
		t.Fatal(err)
	}
	got := st.Get().Diag
	if got.RemoteADB || got.InsecureTLS {
		t.Fatalf("legacy diagnostics still dangerous after migration: %+v", got)
	}
}
