//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepairsStateFileToOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"brain":{"key":"secret-value"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state.json mode = %04o, want 0600", got)
	}
}

func TestWriteCreatesStateFileOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set().Brain().SetKey("secret-value"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state.json mode = %04o, want 0600", got)
	}
}
